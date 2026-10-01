; Рязанцев И.В., А-12-23, 17, v1
$INCLUDE (vars.inc)             ; адреса, биты, константы варианта; в файл для робота mpscode вклеит его текст
DSEG AT 30h
X1:     DS 1                    ; цифра с клавиатуры (0 — Y2 пассивен)
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
org 1bh                         ; Timer1 отсчитал T2 — конец строба Y2
        clr TR1                 ; таймер больше не нужен до следующего вывода
        setb PIN_Y2             ; строб в пассивную 1
        reti
org 23h ; "заглушка" для UART
        nop
        reti
org 2bh
START:
; Инициализация МК **********************************************
        mov SP, #07h
        setb PIN_CSEN           ; включить дешифратор CS — регистр Y2 доступен
        setb PIN_Y2             ; строб Y2 пассивен
        anl TMOD, #0Fh          ; Timer1: режим 1 (16 бит), Timer0 не трогаем
        orl TMOD, #10h
        setb ET1                ; прерывание по концу строба
        setb EA
        mov X1, #3              ; пример входных данных
        mov X2, #200
        call Y2Out ; %proc%
        jmp $ ; %stop%

; Y2Out — вывод Y2 = (G+M+X1+X2) mod 256, при X1 = 0 выводится 0.
; Значение пишется в регистр Y2, затем запускается строб T2 на PIN_Y2;
; процедура не ждёт конца строба — его снимает обработчик Timer1.
; Вход: X1, X2 (ОЗУ). Выход: регистр Y2; PIN_Y2 = 0 на T2 мкс.
; Портит: A, флаги (DPTR сохраняется).
Y2Out:
        mov A, X1
        jz Y2Zero               ; нет цифры с клавиатуры — выводим 0
        add A, X2               ; сумма по модулю 256: перенос просто теряется
        add A, #Y2_CONST        ; плюс G+M варианта
        sjmp Y2Put
Y2Zero:
        clr A
Y2Put:
        push DPL
        push DPH
        mov DPTR, #ADR_Y2
        movx @DPTR, A           ; значение — в регистр до начала строба
        pop DPH
        pop DPL
        clr TR1                 ; перезапуск, если прошлый строб ещё идёт
        mov TH1, #T2_RELOAD_H   ; отсчёт длительности T2
        mov TL1, #T2_RELOAD_L
        clr TF1                 ; не принять старое переполнение за конец строба
        clr PIN_Y2              ; начало строба
        setb TR1
        ret
END
