    SET Y, 0x8000
:main
    IFG Y, 0x8179
    	JSR scrollup
:keyloop
	SET A, [0x9000+Z]
    IFE A, 0
        SET PC, keyloop
    SET [0x9000+Z], 0
    ADD Z, 1
    AND Z, 0xf
    SET PUSH, A
    SET X, Y
    JSR printnum
    SET A, POP
    ADD X, 2
    SET [x], A
    BOR [x], 0x7000 ; white text
    JSR newline
    SET PC, main

; prints the number from A to X
:printnum
    SET PUSH, 0
    IFE A, 0
    	SET PUSH, 0x30 ; 0x30 is '0'
:printnumloop
	IFE A, 0
    	SET PC, printnumend
    SET B, A
    MOD B, 10
    ADD B, 0x30 ; 30 is '0'
    SET PUSH, B
    DIV A, 10
    SET PC, printnumloop
:printnumend
	SET A, POP
	IFE A, 0
    	SET PC, POP
    SET [X], A
    BOR [X], 0x7000 ; white text
    ADD X, 1
    SET PC, printnumend

:newline
	ADD Y, 32
    SET PC, POP

; prints the string pointed to by A to X
:printstr
	IFE [A], 0
    	SET PC, POP
    SET [X], [A]
    BOR [X], 0x7000 ; white text
    ADD X, 1
    ADD A, 1
    SET PC, printstr

; scrolls the entire screen up one row
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
