; Эталон чекера, А-12-23, 16, v1
$INCLUDE (vars.inc)
DSEG AT 30h
Cnt:    DS 2                    ; оставшиеся тики строба, мл./ст.
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
        setb PIN_Y1
        mov TMOD, #11h
        setb ET0
        setb EA
        call Y1Start ; %proc%
        jmp $ ; %stop%

; Y1Start — начало строба Y1 длительностью T1: PIN_Y1 = 0, отсчёт Y1_TICKS тиков Timer0.
; Вход, выход: нет. Конец строба — в T0Tick. Портит: ничего.
Y1Start:
        clr TR0
        mov TH0, #TICK_H
        mov TL0, #TICK_L
        mov Cnt, #LOW(Y1_TICKS)
        mov Cnt+1, #HIGH(Y1_TICKS)
        clr PIN_Y1
        setb TR0
        ret

; T0Tick — обработчик Timer0: перезагрузка на 1 мс, по окончании счёта — конец строба.
T0Tick:
        push PSW
        push ACC
        clr TR0
        mov A, TL0
        add A, #LOW(TICK_L + 7) ; поправка на вход в прерывание и остановку таймера
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
        setb PIN_Y1             ; конец строба
t0_ret: pop ACC
        pop PSW
        reti
END
