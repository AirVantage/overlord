package set

import (
	"testing"
	"strconv"
)

func TestStringSet(t *testing.T) {
	
	cases := []struct {
		init   func(t *testing.T) *Strings
		has	string
		expect  bool
		len	int
	}{
		/* Single instance result */
		{
			init: func(t *testing.T) *Strings {
				ss := NewStringSet()
				return &ss
			},
			has: "12",
			expect: false,
			len: 0,

		},
		/* Single instance result */
		{
			init: func(t *testing.T) *Strings {
				ss := NewStringSet()
				ss.Add("12")
				return &ss
			},
			has: "12",
			expect: true,
			len: 1,
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {

			data := tt.init(t)

			if output := data.Has(tt.has); output != tt.expect {
				t.Errorf("expect %v, got %v", tt.expect, output)
			}

			if output := data.ToSlice(); len(output) != tt.len {
				t.Errorf("expect %v, got %v", tt.len, output)
			}
		})
	}
	
}
/*

*/
