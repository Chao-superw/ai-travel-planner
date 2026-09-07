import pathlib, shutil, subprocess, sys, tempfile, unittest

SCRIPT = pathlib.Path(__file__).with_name('generate_idl.py')
class StableWireIDTest(unittest.TestCase):
    def fixture(self, fields):
        root=pathlib.Path(tempfile.mkdtemp(prefix='travel-idl-test-'))
        (root/'scripts').mkdir();(root/'internal/domain').mkdir(parents=True);(root/'idl').mkdir()
        shutil.copy2(SCRIPT,root/'scripts/generate_idl.py')
        (root/'internal/domain/types.go').write_text('package domain\ntype Request struct {\n'+fields+'\n}\n')
        (root/'idl/model.thrift').write_text('namespace go model\nstruct Request {\n  1: string token\n  2: string username\n}\n')
        return root
    def run_generator(self,root):
        return subprocess.run([sys.executable,str(root/'scripts/generate_idl.py')],capture_output=True,text=True)
    def test_reordering_does_not_reassign_existing_field_ids(self):
        root=self.fixture(' Username string `json:"username"`\n Email string `json:"email"`\n Token string `json:"token"`')
        result=self.run_generator(root);self.assertEqual(result.returncode,0,result.stderr)
        text=(root/'idl/model.thrift').read_text()
        self.assertIn('1: string token',text);self.assertIn('2: string username',text);self.assertIn('3: string email',text)
        again=self.run_generator(root);self.assertEqual(again.returncode,0,again.stderr);self.assertEqual((root/'idl/model.thrift').read_text(),text)
    def test_type_change_is_rejected_without_overwriting_schema(self):
        root=self.fixture(' Token int32 `json:"token"`\n Username string `json:"username"`')
        previous=(root/'idl/model.thrift').read_text();result=self.run_generator(root)
        self.assertNotEqual(result.returncode,0);self.assertEqual((root/'idl/model.thrift').read_text(),previous)
if __name__=='__main__': unittest.main()
