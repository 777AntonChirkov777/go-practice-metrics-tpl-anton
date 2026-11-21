package c

import "example.com/foreigngen/a"

func Clear(x *a.A) {
	x.Reset()
}
