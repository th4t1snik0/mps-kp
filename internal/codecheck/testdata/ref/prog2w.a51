; Эталон чекера, А-12-23, 15, v1
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
        mov HEAD_L, #LOW(BUF_START)
        mov HEAD_H, #HIGH(BUF_START)
        mov TAIL_L, #LOW(BUF_START)
        mov TAIL_H, #HIGH(BUF_START)
        setb F_EMPTY
        clr F_OVF
        mov A, #55h
        call BufWrite ; %proc%
        jmp $ ; %stop%

; BufWrite — запись отсчёта в кольцевой буфер; при переполнении затирается самый старый.
; Вход: A — отсчёт. Выход: F_EMPTY сброшен; F_OVF = 1, если отсчёт затёр старый.
; Портит: A, B, DPTR, PSW.
BufWrite:
        mov B, A
        jb F_EMPTY, bw_put      ; пустой — точно есть место
        mov A, HEAD_L
        cjne A, TAIL_L, bw_put
        mov A, HEAD_H
        cjne A, TAIL_H, bw_put
        mov DPL, TAIL_L         ; полон: выбросить самый старый — сдвинуть хвост
        mov DPH, TAIL_H
        call BufNext
        mov TAIL_L, DPL
        mov TAIL_H, DPH
        setb F_OVF
bw_put: mov DPL, HEAD_L
        mov DPH, HEAD_H
        mov A, B
        movx @DPTR, A
        call BufNext
        mov HEAD_L, DPL
        mov HEAD_H, DPH
        clr F_EMPTY
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
