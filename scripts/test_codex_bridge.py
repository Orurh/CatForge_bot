import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import subprocess

spec = importlib.util.spec_from_file_location('bridge', Path(__file__).with_name('codex_bridge.py'))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


class BridgeTests(unittest.TestCase):
    def test_quota_persists_across_connections(self):
        with tempfile.TemporaryDirectory() as temp:
            db = Path(temp) / 'quota.sqlite'
            self.assertTrue(bridge.reserve(db, 2))
            self.assertTrue(bridge.reserve(db, 2))
            self.assertFalse(bridge.reserve(db, 2))

    @patch.object(bridge.subprocess, 'Popen')
    def test_usage_and_output(self, popen):
        proc = popen.return_value
        proc.returncode = 0
        proc.communicate.return_value = ('{"type":"item.completed","item":{"type":"agent_message","text":"Мяу"}}\n'
                                         '{"type":"turn.completed","usage":{"input_tokens":123,"output_tokens":5}}', None)
        result = bridge.generate('/codex', '/empty', {'instructions': 'cat', 'input': 'hello'})
        self.assertEqual(result['usage']['input_tokens'], 123)
        self.assertEqual(result['output'][0]['content'][0]['text'], 'Мяу')
        self.assertNotIn('AI_API_KEY', popen.call_args.kwargs['env'])

    @patch.object(bridge.os, 'killpg')
    @patch.object(bridge.subprocess, 'Popen')
    def test_timeout_kills_process_group(self, popen, kill):
        proc = popen.return_value
        proc.pid = 42
        proc.communicate.side_effect = [subprocess.TimeoutExpired('codex', 25), ('', None)]
        with self.assertRaises(RuntimeError):
            bridge.generate('/codex', '/empty', {'instructions': 'cat', 'input': 'hello'})
        kill.assert_called_once_with(42, bridge.signal.SIGKILL)


if __name__ == '__main__':
    unittest.main()
