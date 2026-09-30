; Эталон чекера, А-12-23, 14, v1
$INCLUDE (vars.inc)
DSEG AT 30h
X1:     DS 1                    ; цифра с клавиатуры
X2:     DS 1                    ; отсчёт из буфера
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
org 1bh                         ; Timer1: конец строба Y2
        clr TR1
        setb PIN_Y2
        reti
org 23h ; "заглушка" для UART
        nop
        reti
org 2bh
START:
; Инициализация МК **********************************************
        mov SP, #07h
        setb PIN_CSEN
        setb PIN_Y2             ; строб пассивен
        mov TMOD, #11h          ; Timer0, Timer1 — 16 бит
        setb ET1
        setb EA
        mov X1, #3
        mov X2, #200
        call Y2Out ; %proc%
        jmp $ ; %stop%

; Y2Out — расчёт Y2 = (G+M+X1+X2) mod 256 при X1 ≠ 0, иначе 0; запись в регистр Y2 и строб T2.
; Вход: X1, X2 в ОЗУ. Выход: регистр Y2, строб PIN_Y2 = 0 на T2 (конец — прерывание Timer1).
; Портит: A, DPTR, PSW.
Y2Out:
        mov A, X1
        jz y2_wr                ; X1 = 0 — пассивное Y2 = 0
        mov A, #Y2_CONST
        add A, X1
        add A, X2               ; перенос отбрасывается — это и есть mod 256
y2_wr:  mov DPTR, #ADR_Y2
        movx @DPTR, A
        clr TR1
        mov TH1, #T2_RELOAD_H
        mov TL1, #T2_RELOAD_L
        clr PIN_Y2              ; начало строба сопровождения
        setb TR1
        ret
END
