#shared: {
	a?: string
}

#archA: #shared & {
	b?: string
}

#archB: #shared & {
	c?: string
}

#conbined: #archA & #archB
