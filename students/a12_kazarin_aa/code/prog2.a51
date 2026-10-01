; Казарин А.А., А-12-23, 8, v1
$INCLUDE (vars.inc)             ; адреса, биты, константы варианта; в файл для робота mpscode вклеит его текст
DSEG AT 30h
; переменных нет — указатели и флаги буфера заданы в vars.inc
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
        setb PIN_CSEN           ; разрешить дешифратор CS — иначе буфер не выбирается
        mov HEAD_L, #LOW(BUF_START)     ; голова и хвост — в начале буфера
        mov HEAD_H, #HIGH(BUF_START)
        mov TAIL_L, #LOW(BUF_START)
        mov TAIL_H, #HIGH(BUF_START)
        setb F_EMPTY            ; буфер пуст
        clr F_OVF               ; и не переполнен
        call BufRead ; %proc%
        jmp $ ; %stop%

; BufRead — чтение самого старого отсчёта из кольцевого буфера (по «хвосту»).
; Вход: TAIL — адрес самого старого отсчёта, HEAD — адрес следующей записи, F_EMPTY, F_OVF.
; Выход: A — отсчёт; хвост сдвинут на следующую ячейку (после последней — на BUF_START);
; F_OVF = 0; F_EMPTY = 1, если хвост догнал голову. Буфер пуст — A = 0, указатели и флаги не меняются.
; Портит: A, PSW (DPTR сохраняется).
BufRead:
        jnb F_EMPTY, BrHave     ; есть данные — читаем
        clr A                   ; пусто: вернуть 0, ничего не трогать
        ret
BrHave:
        push DPL
        push DPH
        mov DPL, TAIL_L         ; адрес самого старого отсчёта
        mov DPH, TAIL_H
        movx A, @DPTR           ; забрать отсчёт из IDT7005
        push ACC                ; сохранить до выхода — A ещё нужен для сравнений
        inc DPTR                ; хвост на следующую ячейку
        mov A, DPL
        cjne A, #LOW(BUF_END), BrSave   ; вышли за последнюю ячейку? сравнить оба байта
        mov A, DPH
        cjne A, #HIGH(BUF_END), BrSave
        mov DPTR, #BUF_START    ; заворот хвоста на начало кольца
BrSave:
        mov TAIL_L, DPL
        mov TAIL_H, DPH
        clr F_OVF               ; одна ячейка освободилась — буфер уже не полон
        mov A, TAIL_L
        cjne A, HEAD_L, BrDone  ; хвост не догнал голову — данные ещё есть
        mov A, TAIL_H
        cjne A, HEAD_H, BrDone
        setb F_EMPTY            ; прочитали последний отсчёт
BrDone:
        pop ACC                 ; вернуть прочитанный отсчёт
        pop DPH
        pop DPL
        ret
END
