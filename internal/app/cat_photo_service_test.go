package app

import "testing"

func TestCatPhotoFileIDValidation(t *testing.T) {
    for _, tc := range []struct{input string; valid bool}{
        {"",false},{"a",true},{"abc def",true},{" abc",false},{"abc ",false},
        {string(make([]byte,513)),false},
    } {
        if got:=ValidCatPhotoFileID(tc.input);got!=tc.valid {t.Errorf("file id length=%d got %v want %v",len(tc.input),got,tc.valid)}
    }
}
