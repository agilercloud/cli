"""Exercise the release workflow's real Bash installer using local archives."""
import hashlib
import io
import os
import pathlib
import re
import subprocess
import tarfile
import tempfile
import textwrap
import unittest


class ReleaseInstallerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        workflow = pathlib.Path(__file__).with_name('workflows') / 'release.yml'
        text = workflow.read_text()
        # Deliberately narrow extraction of this named literal Bash step, not a
        # general YAML parser. Fail if its shape changes rather than test stale code.
        step = text.split('      - name: Install verified GoReleaser\n', 1)[1]
        step = step.split('\n      - name:', 1)[0]
        header, block = step.split('        run: |\n', 1)
        assert '        shell: bash\n' in header
        cls.pins = dict(re.findall(r"          (TOOL_\w+): '([^']+)'\n", header))
        assert re.fullmatch(r'\d+\.\d+\.\d+', cls.pins['TOOL_VERSION'])
        assert re.fullmatch(r'[0-9a-f]{64}', cls.pins['TOOL_SHA256'])
        cls.script = textwrap.dedent(block)
        assert 'goreleaser release --clean' in text
        assert 'goreleaser/goreleaser-action@' not in text

    def exercise(self, case):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            tools = root / 'tools'
            tools.mkdir()
            runner = root / 'runner'
            runner.mkdir()
            archive = root / 'fixture.tar.gz'
            payload = b'fixture bytes, never executed\n'
            member = 'other' if case == 'missing-member' else 'goreleaser'
            with tarfile.open(archive, 'w:gz') as output:
                entry = tarfile.TarInfo(member)
                entry.size = len(payload)
                output.addfile(entry, io.BytesIO(payload))
                extra = tarfile.TarInfo('not-installed')
                extra.size = len(payload)
                output.addfile(extra, io.BytesIO(payload))
            digest = hashlib.sha256(archive.read_bytes()).hexdigest()
            if case == 'mismatch':
                digest = '0' * 64
            elif case == 'invalid-digest':
                digest = ''
            curl = tools / 'curl'
            curl.write_text('''#!/bin/bash
set -eu
printf '%s\\n' "$@" > "$CURL_ARGS"
[[ "$DOWNLOAD_FAIL" == no ]] || exit 22
while [[ $# -gt 0 ]]; do
  if [[ "$1" == --output ]]; then cp "$FIXTURE" "$2"; exit 0; fi
  shift
done
exit 2
''')
            curl.chmod(0o755)
            # A previously installed executable must not turn failure into success.
            stale = tools / 'goreleaser'
            stale.write_text('#!/bin/bash\ntouch "$EXECUTED"\nexit 0\n')
            stale.chmod(0o755)
            path_file = root / 'github-path'
            path_file.touch()
            env = dict(os.environ)
            env.update(self.pins)
            env.update(TOOL_SHA256=digest, RUNNER_OS='Linux',
                       RUNNER_ARCH='ARM64' if case == 'unsupported' else 'X64',
                       RUNNER_TEMP=str(runner), GITHUB_PATH=str(path_file),
                       PATH=str(tools) + os.pathsep + os.environ['PATH'],
                       FIXTURE=str(archive), CURL_ARGS=str(root / 'curl-args'),
                       DOWNLOAD_FAIL='yes' if case == 'download-failure' else 'no',
                       EXECUTED=str(root / 'executed'))
            result = subprocess.run(['bash', '-c', self.script], env=env,
                                    capture_output=True, text=True)
            self.assertFalse((root / 'executed').exists())
            self.assertFalse(list(runner.glob('*/archive.tar.gz')))
            if case == 'success':
                self.assertEqual(result.returncode, 0, result.stderr)
                installed = pathlib.Path(path_file.read_text().strip())
                self.assertEqual(installed.parent, runner)
                self.assertEqual((installed / 'goreleaser').read_bytes(), payload)
                self.assertTrue(os.access(installed / 'goreleaser', os.X_OK))
                self.assertFalse((installed / 'not-installed').exists())
                args = (root / 'curl-args').read_text().splitlines()
                for flag in ('--fail', '--location', '--connect-timeout', '--max-time'):
                    self.assertIn(flag, args)
                for flag in ('--proto', '--proto-redir'):
                    self.assertEqual(args[args.index(flag) + 1], '=https')
                self.assertIn(
                    f'https://github.com/goreleaser/goreleaser/releases/download/v{self.pins["TOOL_VERSION"]}/goreleaser_Linux_x86_64.tar.gz',
                    args)
            else:
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(path_file.read_text(), '')
                self.assertFalse(list(runner.glob('*/goreleaser')))
                if case in ('unsupported', 'invalid-digest'):
                    self.assertFalse((root / 'curl-args').exists())

    def test_installer(self):
        for case in ('success', 'mismatch', 'invalid-digest', 'download-failure',
                     'missing-member', 'unsupported'):
            with self.subTest(case=case):
                self.exercise(case)


if __name__ == '__main__':
    unittest.main()
