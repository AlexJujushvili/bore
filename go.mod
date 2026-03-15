module test_bore_digital

go 1.25.1

require github.com/jkuri/bore v0.5.0

require (
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/jkuri/bore => ./_bore
