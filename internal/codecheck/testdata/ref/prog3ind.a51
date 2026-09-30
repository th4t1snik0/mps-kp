; Эталон чекера, А-12-23, 15, v1
$INCLUDE (vars.inc)
DSEG AT 30h
Cnt:    DS 2                    ; оставшиеся тики индикации
CSEG AT 0h
        sjmp START
org 03h ; "заглушка" для INT0
        nop
        reti
org 0bh                         ; Timer0: тик 1 мс
        ljmp T0Tick
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
        mov DPTR, #ADR_IND
        mov A, #IND_OFF
        movx @DPTR, A           ; индикатор погашен
        mov TMOD, #11h
        setb ET0
        setb EA
        mov A, #5
        call Show ; %proc%
        jmp $ ; %stop%

; Show — вывод символа на индикатор на время T3 (гашение — в T0Tick).
; Вход: A — код символа (0–9 — цифра, 10 — «E», больше — погасить). Портит: A, DPTR, PSW.
Show:
        cjne A, #11, $+3
        jc sh_ok                ; код < 11
        mov A, #11              ; «погасить»
sh_ok:  mov DPTR, #SegTab
        movc A, @A+DPTR
        xrl A, #IND_OFF         ; у общего анода сегмент горит нулём
        mov DPTR, #ADR_IND
        movx @DPTR, A
        clr TR0
        mov TH0, #TICK_H
        mov TL0, #TICK_L
        mov Cnt, #LOW(T3_TICKS)
        mov Cnt+1, #HIGH(T3_TICKS)
        setb TR0
        ret

; T0Tick — обработчик Timer0: тик 1 мс, по окончании T3 индикатор гасится.
T0Tick:
        push PSW
        push ACC
        push DPL
        push DPH
        clr TR0
        mov A, TL0
        add A, #LOW(TICK_L + 7)
        mov TL0, A
        mov A, TH0
        addc A, #TICK_H
        mov TH0, A
        setb TR0
        mov A, Cnt
        jnz t0_lo
        dec Cnt+1
t0_lo:  dec Cnt
        mov A, Cnt
        orl A, Cnt+1
        jnz t0_ret
        clr TR0
        mov DPTR, #ADR_IND
        mov A, #IND_OFF
        movx @DPTR, A           ; время индикации вышло
t0_ret: pop DPH
        pop DPL
        pop ACC
        pop PSW
        reti

; Сегменты (общий катод, D0…D7 = a…g, dp): 0–9, E, пусто
SegTab: DB 3Fh, 06h, 5Bh, 4Fh, 66h, 6Dh, 7Dh, 07h, 7Fh, 6Fh, 79h, 00h
END
