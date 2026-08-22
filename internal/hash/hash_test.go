package hash

import "testing"

const (
	rfc4231Key  = "Jefe"
	rfc4231Data = "what do ya want for nothing?"
	rfc4231Sum  = "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"
)

func TestSum_KnownVector(t *testing.T) {
	if got := Sum(rfc4231Key, []byte(rfc4231Data)); got != rfc4231Sum {
		t.Errorf("Sum = %q, want %q", got, rfc4231Sum)
	}
}

func TestSum_DependsOnKey(t *testing.T) {
	body := []byte(`{"id":"Alloc","type":"gauge","value":1}`)

	if Sum("secret", body) == Sum("invalidkey", body) {
		t.Error("подпись обязана меняться вместе с ключом")
	}
}

func TestEqual(t *testing.T) {
	body := []byte(rfc4231Data)

	cases := []struct {
		name string
		key  string
		body []byte
		got  string
		want bool
	}{
		{"верная подпись", rfc4231Key, body, rfc4231Sum, true},
		{"верхний регистр hex", rfc4231Key, body, "5BDCC146BF60754E6A042426089575C75A003F089D2739839DEC58B964EC3843", true},
		{"чужой ключ", "invalidkey", body, rfc4231Sum, false},
		{"тело изменено", rfc4231Key, []byte(rfc4231Data + "!"), rfc4231Sum, false},
		{"не шестнадцатеричная строка", rfc4231Key, body, "not-a-hash", false},
		{"пустая строка", rfc4231Key, body, "", false},
		{"обрезанная подпись", rfc4231Key, body, rfc4231Sum[:32], false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Equal(c.key, c.body, c.got); got != c.want {
				t.Errorf("Equal = %v, want %v", got, c.want)
			}
		})
	}
}
