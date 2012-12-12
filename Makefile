all: smtpc smtpc.32

smtpc.32: smtpc.go
	gccgo -O3 -o $@ $< -static-libgcc 

smtpc: smtpc.go
	gccgo -o $@ $< -static-libgcc

clean:
	rm smtpc smtpc.32
