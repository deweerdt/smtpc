all: smtpc smtpc.32

smtpc.32: smtpc.go
	gccgo -o $@ $< -static-libgcc  -static-libgo

smtpc: smtpc.go
	gccgo -o $@ $< -static-libgcc  -static-libgo

clean:
	rm smtpc smtpc.32
