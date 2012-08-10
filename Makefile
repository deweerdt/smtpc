all: smtpc

smtpc: smtpc.go
	gccgo -o $@ $<
