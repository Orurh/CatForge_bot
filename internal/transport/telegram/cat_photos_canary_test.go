package telegram

import "testing"

func TestCatPhotoCanaryAllows(t *testing.T) {
    tests := []struct {
        name string
        canaryID int64
        telegramID int64
        want bool
    }{
        {"unrestricted", 0, 100, true},
        {"matching tester", 100, 100, true},
        {"another user", 100, 101, false},
    }
    for _, tc := range tests {
        t.Run(tc.name,func(t *testing.T){
            if got:=catPhotoCanaryAllows(tc.canaryID,tc.telegramID);got!=tc.want {
                t.Fatalf("catPhotoCanaryAllows(%d, %d) = %v, want %v",tc.canaryID,tc.telegramID,got,tc.want)
            }
        })
    }
}
