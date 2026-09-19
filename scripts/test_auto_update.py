import copy
import unittest

from auto_update import successful_run


class SelectionTests(unittest.TestCase):
    def test_only_successful_main_push_from_own_repository_is_eligible(self):
        run = dict(head_sha='a' * 40, head_branch='main', event='push', status='completed',
                   conclusion='success', head_repository={'full_name': 'Orurh/CatForge_bot'})
        self.assertTrue(successful_run([run], 'a' * 40, 'Orurh/CatForge_bot'))
        for key, value in [('head_sha', 'b' * 40), ('head_branch', 'feature'),
                           ('event', 'pull_request'), ('event', 'workflow_dispatch'),
                           ('status', 'in_progress'), ('conclusion', 'failure'),
                           ('conclusion', 'cancelled'),
                           ('head_repository', {'full_name': 'someone/fork'})]:
            with self.subTest(key=key, value=value):
                wrong = copy.deepcopy(run)
                wrong[key] = value
                self.assertFalse(successful_run([wrong], 'a' * 40, 'Orurh/CatForge_bot'))
        self.assertFalse(successful_run([], 'a' * 40, 'Orurh/CatForge_bot'))
