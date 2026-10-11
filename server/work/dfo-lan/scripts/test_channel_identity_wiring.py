"""Exercise launch wiring without starting services or touching player data."""
import ast
import pathlib
import runpy
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[4]
LAUNCH = pathlib.Path(__file__).with_name('launch_local.py')


class ChannelIdentityWiring(unittest.TestCase):
    def test_local_flag_without_profile_argument(self):
        tree = ast.parse(LAUNCH.read_text(encoding='utf-8-sig'))
        main = next(n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name == 'main')
        selected = []
        for node in main.body:
            if not isinstance(node, ast.Assign):
                continue
            for target in node.targets:
                if isinstance(target, ast.Name) and target.id == 'channel_identity':
                    selected.append(node)
                elif (isinstance(target, ast.Subscript) and isinstance(target.slice, ast.Constant)
                      and target.slice.value == 'DFO_CHANNEL_IDENTITY'):
                    selected.append(node)
        self.assertEqual(len(selected), 2)
        code = compile(ast.fix_missing_locations(ast.Module(body=selected, type_ignores=[])), str(LAUNCH), 'exec')
        for local, expected in [({}, '0'), ({'channel_identity': True}, '1'), ({'channel_identity': False}, '0')]:
            scope = {'local': local, 'env': {}}
            exec(code, scope)
            self.assertEqual(scope['env']['DFO_CHANNEL_IDENTITY'], expected)

    def test_stop_includes_candidate_without_stopping_anything(self):
        scope = runpy.run_path(str(ROOT / 'stop_environment.py'))
        calls = []
        stop = scope['stop_wireprobe']
        stop.__globals__['kill_by_image'] = calls.append
        stop()
        self.assertIn('wireprobe-channel-identity-candidate.exe', calls)
        self.assertIn('wireprobe-handoff-source.exe', calls)
        self.assertIn('wireprobe-pvf.exe', calls)


if __name__ == '__main__':
    unittest.main()
