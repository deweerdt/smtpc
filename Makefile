all: smtpc smtpc.32

smtpc.32: smtpc.go
	gccgo -o $@ $< -static-libgcc

smtpc: smtpc.go
	gccgo -o $@ $< -static-libgcc
