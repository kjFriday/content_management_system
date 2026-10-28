// ==== Generic Company Inc. portfolio ====================================
// Project: content_management_system | File: icon_test.go
// About: ==== Generic Company Inc. portfolio ===============================================...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// ==== Generic Company Inc. portfolio ===============================================
// Project: content_management_system | File: icon_test.go
// About: Tests for package main.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image/png"
	"os"
	"regexp"
	"testing"
)

// Every size is the 16 px Silk original enlarged by whole pixels, and the files match the credits table.
func TestPixelExactAndCredited(t *testing.T) {
	PNG32, _ := icons.ReadFile("icon/app-32.png")
	PNG256, _ := icons.ReadFile("icon/app-256.png")
	ICO, _ := icons.ReadFile("icon/app.ico")
	raw, err := os.ReadFile("icon/silk-newspaper.png")
	if err != nil {
		t.Fatal(err)
	}
	small, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	for k, b := range map[int][]byte{2: PNG32, 16: PNG256} {
		big, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if big.Bounds().Dx() != 16*k {
			t.Fatalf("x%d: %v", k, big.Bounds())
		}
		for y := 0; y < 16*k; y++ {
			for x := 0; x < 16*k; x++ {
				r1, g1, b1, a1 := big.At(x, y).RGBA()
				r2, g2, b2, a2 := small.At(x/k, y/k).RGBA()
				if a1 != a2 || (a1 != 0 && (r1 != r2 || g1 != g2 || b1 != b2)) {
					t.Fatalf("x%d pixel (%d,%d) differs", k, x, y)
				}
			}
		}
	}
	if binary.LittleEndian.Uint16(ICO[4:]) != 4 {
		t.Fatal("app.ico should hold 16, 32, 48 and 256 px")
	}
	readme, _ := os.ReadFile("icon/README.md")
	if !bytes.Contains(readme, []byte("Creative Commons Attribution 2.5")) {
		t.Fatal("credit missing")
	}
	rows := regexp.MustCompile("\\| `([^`]+)` \\|[^|]*\\| `([0-9a-f]{64})` \\|").FindAllSubmatch(readme, -1)
	if len(rows) != 6 {
		t.Fatalf("%d files listed", len(rows))
	}
	for _, r := range rows {
		b, err := os.ReadFile("icon/" + string(r[1]))
		sum := sha256.Sum256(b)
		if err != nil || hex.EncodeToString(sum[:]) != string(r[2]) {
			t.Fatalf("%s does not match its SHA-256", r[1])
		}
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=576a12a856ff954f3571b16235a24e1bd2b98b94327d84f2854a9ba077d107a6 ====
