import "time"

#TestSA: {
	field1: string
	field2: int
	filed3: {
		field1: string
		field2: int
		field3: time.Time
	}
}

// mysql config validator
#Exemplar: {
	l: {
		name: string
	}
	sa: #TestSA

	ta: {
		field1: string
		field2: int
	}
}

[N=string]: #Exemplar & {
	l: name: N
}

// configuration require
configuration: #Exemplar & {
}
