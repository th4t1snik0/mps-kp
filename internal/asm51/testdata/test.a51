; Рязанцев ИВ, А-12-22, 14, v1
$INCLUDE (vars.inc)
DSEG AT 30h
cnt:    DS 1
buf:    DS 4
CSEG AT 0h
        sjmp START
org 03h ; INT0
        inc R5
        reti
org 0bh
        nop
        reti
org 13h
        nop
        reti
org 1bh
        nop
        reti
org 23h
        nop
        reti
org 2bh
START:
        mov SP, #07h
        setb EX0
        setb IT0
        setb EA
        call YourFunc; %proc%
        jmp $; %stop%

; YourFunc: пишет столбец в клавиатуру, читает строки, копирует 4 байта XRAM->IRAM
YourFunc:
        clr F_EMPTY
        setb F_OVF
        clr PIN_Y1
        mov DPTR, #ADR_KB
        mov A, #0FEh
        movx @DPTR, A
        movx A, @DPTR
        mov buf, A
        mov DPTR, #BUF_START
        mov R0, #buf+1
        mov cnt, #3
loop:   movx A, @DPTR
        mov @R0, A
        inc DPTR
        inc R0
        djnz cnt, loop
        mov DPTR, #ADR_Y2
        mov A, #Y2_CONST
        movx @DPTR, A
        setb PIN_Y1
        call Helper
        ret
Helper: mov HEAD_L, #LOW(BUF_START)
        mov HEAD_H, #HIGH(BUF_START)
        ret
TBL:    DB 3Fh, 06h, 5Bh
END
