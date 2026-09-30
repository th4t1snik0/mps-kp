; Эталон чекера, А-12-23, 14, v1
$INCLUDE (vars.inc)
DSEG AT 30h
CSEG AT 0h
        sjmp START
org 03h ; "заглушка" для INT0
        nop
        reti
org 0bh ; "заглушка" для Tmr0_ovf
        nop
        reti
org 13h ; "заглушка" для INT1
        nop
        reti
org 1bh ; "заглушка" для Tmr1_ovf
        nop
        reti
org 23h ; "заглушка" для UART
        nop
        reti
org 2bh
START:
; Инициализация МК **********************************************
        mov SP, #07h
        setb PIN_CSEN
        mov HEAD_L, #LOW(BUF_START)     ; буфер пуст: голова = хвост = начало
        mov HEAD_H, #HIGH(BUF_START)
        mov TAIL_L, #LOW(BUF_START)
        mov TAIL_H, #HIGH(BUF_START)
        setb F_EMPTY
        clr F_OVF
        call BufRead ; %proc%
        jmp $ ; %stop%

; BufRead — чтение самого старого отсчёта из кольцевого буфера.
; Вход: нет. Выход: A — отсчёт (0, если буфер пуст). Флаги: F_EMPTY — буфер опустел, F_OVF сброшен.
; Портит: DPTR, PSW.
BufRead:
        jb F_EMPTY, br_empty
        mov DPL, TAIL_L
        mov DPH, TAIL_H
        movx A, @DPTR
        push ACC
        call BufNext
        mov TAIL_L, DPL
        mov TAIL_H, DPH
        clr F_OVF               ; после чтения буфер не полон
        mov A, DPL
        cjne A, HEAD_L, br_ret
        mov A, DPH
        cjne A, HEAD_H, br_ret
        setb F_EMPTY            ; хвост догнал голову
br_ret: pop ACC
        ret
br_empty:
        clr A
        ret

; BufNext — DPTR на следующую ячейку буфера с заворотом BUF_END → BUF_START. Портит A.
BufNext:
        inc DPTR
        mov A, DPL
        cjne A, #LOW(BUF_END), bn_ret
        mov A, DPH
        cjne A, #HIGH(BUF_END), bn_ret
        mov DPTR, #BUF_START
bn_ret: ret
END
