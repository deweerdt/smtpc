all: smtpc

smtpc: smtpc.8
	8l -o $@ $<

smtpc.8: smtpc.go
	8g $<
