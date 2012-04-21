	SET I, 1
    SET Y, 0x8000
:main
	IFG I, 100
		SET PC, end
    IFG Y, 0x8179
    	JSR scrollup
	SET A, I
	MOD A, 15
	IFE A, 0
		SET PC, fizzbuzz
	SET A, I
	MOD A, 5
	IFE A, 0
		SET PC, buzz
	SET A, I
	MOD A, 3
	IFE A, 0
		SET PC, fizz

; print number
	SET A, I
    SET X, Y
    SET PUSH, 0
:loop
	IFE A, 0
    	SET PC, print
    SET B, A
    MOD B, 10
    ADD B, 0x30 ; 30 is '0'
    SET PUSH, B
    DIV A, 10
    SET PC, loop
:print
	SET A, POP
	IFE A, 0
    	SET PC, newline
    SET [X], A
    BOR [X], 0x7000 ; white text
    ADD X, 1
    SET PC, print
:newline
	ADD Y, 32
    ADD I, 1
    SET PC, main

:printstr
	SET X, Y
:printstrloop
	IFE [A], 0
    	SET PC, newline
    SET [X], [A]
    BOR [X], 0x7000 ; white text
    ADD X, 1
    ADD A, 1
    SET PC, printstrloop

:fizz
	SET A, fizzstr
    SET PC, printstr
:buzz
	SET A, buzzstr
    SET PC, printstr
:fizzbuzz
	SET A, fizzbuzzstr
    set PC, printstr

:fizzstr
	DAT "Fizz", 0
:buzzstr
	DAT "Buzz", 0
:fizzbuzzstr
	DAT "FizzBuzz", 0

:scrollup
	SUB Y, 32
    SET A, 0x8000
    SET B, 0x8020
:scrollloop
	IFG B, 0x8179
    	SET PC, scrollend
    SET [A], [B]
    ADD A, 1
    ADD B, 1
    SET PC, scrollloop
:scrollend
	; clear the last line
    IFG A, 0x8179
    	SET PC, POP
    SET [A], 0
    ADD A, 1
    SET PC, scrollend

:end
	SET PC, end

