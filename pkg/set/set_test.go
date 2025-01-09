package set

import (
	"strconv"
	"testing"
)

func TestGenSetString(t *testing.T) {

	cases := []struct {
		init   func(t *testing.T) *Set[string]
		has    string
		expect bool
		len    int
	}{
		/* Empty set */
		{
			init: func(t *testing.T) *Set[string] {
				return New[string]()
			},
			has:    "12",
			expect: false,
			len:    0,
		},
		/* One element */
		{
			init: func(t *testing.T) *Set[string] {
				ss := New[string]()
				ss.Add("12")
				return ss
			},
			has:    "12",
			expect: true,
			len:    1,
		},
		/* Two elements */
		{
			init: func(t *testing.T) *Set[string] {
				ss := New[string]()
				ss.Add("12")
				ss.Add("13")
				return ss
			},
			has:    "12",
			expect: true,
			len:    2,
		},
		/* Two element, three adds */
		{
			init: func(t *testing.T) *Set[string] {
				ss := New[string]()
				ss.Add("12")
				ss.Add("12")
				ss.Add("13")
				return ss
			},
			has:    "12",
			expect: true,
			len:    2,
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

func TestGenSetInt(t *testing.T) {

	cases := []struct {
		init   func(t *testing.T) *Set[int]
		has    int
		expect bool
		len    int
	}{
		/* Single instance result */
		{
			init: func(t *testing.T) *Set[int] {
				return New[int]()
			},
			has:    12,
			expect: false,
			len:    0,
		},
		/* Single instance result */
		{
			init: func(t *testing.T) *Set[int] {
				ss := New[int]()
				ss.Add(12)
				return ss
			},
			has:    12,
			expect: true,
			len:    1,
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
