package bit_field

import "testing"

type bitfieldTest struct {
	length int64
	set []int64
	result []byte
}

var bitfieldTests = []bitfieldTest{
	bitfieldTest{8, []int64{0, 7}, []byte{129}},
	bitfieldTest{8, []int64{1, 5}, []byte{68}},
}


func TestBitfield(t *testing.T) {
	for _, bt := range bitfieldTests {
		b := NewBitfield(bt.length)
		for _, i := range bt.set {
			b.Set(i)
		}
		for i := int64(0); i < bt.length; i++ {
			if isInArray(bt.set, i) && !b.IsSet(i) {
				t.Errorf("Bitfield[%d] = false, expected true", i)
			} else if !isInArray(bt.set, i) && b.IsSet(i) {
				t.Errorf("Bitfield[%d] = true, expected false", i)
			}
		}
		bytes := b.Bytes()
		for i, expected_bytes := range bt.result {
			if expected_bytes != bytes[i] {
				t.Errorf("Got %q, expected %q for position %d", b.Bytes(), bt.result, i)
			}
		}
	}
}

func TestBitfieldFromBytes(t *testing.T) {
	for _, bt := range bitfieldTests {
		b, err := NewBitfieldFromBytes(bt.length, bt.result)
		if err != nil {
			t.Errorf("Unable to initialize bitfield from given bytes: %q", bt.result);
		}
		for i := int64(0); i < bt.length; i++ {
			if isInArray(bt.set, i) && !b.IsSet(i) {
				t.Errorf("Bitfield[%d] = false, expected true", i)
			} else if !isInArray(bt.set, i) && b.IsSet(i) {
				t.Errorf("Bitfield[%d] = true, expected false", i)
			}
		}
		bytes := b.Bytes()
		for i, expected_bytes := range bt.result {
			if expected_bytes != bytes[i] {
				t.Errorf("Got %q, expected %q for position %d", b.Bytes(), bt.result, i)
			}
		}
	}
}

func isInArray(array []int64, value int64) bool {
	for _, i := range array {
		if i == value {
			return true
		}
	}
	return false
}