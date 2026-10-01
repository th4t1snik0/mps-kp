; Рязанцев И.В., А-12-23, 17, v1
$INCLUDE (vars.inc)             ; адреса, биты, константы варианта; в файл для робота mpscode вклеит его текст
DSEG AT 30h
; указатели и флаги буфера — по адресам из ТЗ (HEAD_*, TAIL_*, F_EMPTY, F_OVF в vars.inc)
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
        mov SP, #07h            ; стек сразу за банком 0 (ТЗ: SP = 07h)
        setb PIN_CSEN           ; разрешить дешифратор адреса — буфер выбирается только при CS_EN = 1
        mov HEAD_L, #LOW(BUF_START)  ; буфер пуст: голова и хвост — в первой ячейке
        mov HEAD_H, #HIGH(BUF_START)
        mov TAIL_L, #LOW(BUF_START)
        mov TAIL_H, #HIGH(BUF_START)
        setb F_EMPTY            ; данных нет
        clr F_OVF               ; и ничего не затёрто
        mov A, #5Ah             ; пример отсчёта X2
        call BufPut ; %proc%
        jmp $ ; %stop%

; BufPut — запись отсчёта в кольцевой буфер (двухпортовое ОЗУ IDT7005).
; Голова — адрес ячейки для следующей записи, хвост — для следующего чтения; после BUF_END−1 идёт BUF_START.
; Вход: A — отсчёт.
; Выход: отсчёт в буфере, голова сдвинута, F_EMPTY = 0; если буфер был полон — самый старый отсчёт
;        выброшен (хвост сдвинут) и F_OVF = 1. Портит: A, B, DPTR, PSW.
BufPut:
        mov B, A                ; сохранить отсчёт — A нужен для сравнений
        jb F_EMPTY, bp_put      ; пустой буфер не может быть полным
        mov A, HEAD_L
        cjne A, TAIL_L, bp_put  ; голова не догнала хвост — место есть
        mov A, HEAD_H
        cjne A, TAIL_H, bp_put
        mov DPL, TAIL_L         ; буфер полон: самый старый отсчёт уходит —
        mov DPH, TAIL_H
        call BufNext            ; хвост на следующую ячейку
        mov TAIL_L, DPL
        mov TAIL_H, DPH
        setb F_OVF              ; отметить переполнение
bp_put: mov DPL, HEAD_L
        mov DPH, HEAD_H
        mov A, B
        movx @DPTR, A           ; отсчёт в ячейку головы
        call BufNext            ; голова на следующую ячейку
        mov HEAD_L, DPL
        mov HEAD_H, DPH
        clr F_EMPTY             ; в буфере теперь есть данные
        ret

; BufNext — DPTR на следующую ячейку буфера; после последней (BUF_END−1) — снова BUF_START.
; Вход, выход: DPTR. Портит: A.
BufNext:
        inc DPTR
        mov A, DPL
        xrl A, #LOW(BUF_END)    ; ноль — младший байт совпал с концом буфера
        jnz bn_ret
        mov A, DPH
        xrl A, #HIGH(BUF_END)
        jnz bn_ret
        mov DPTR, #BUF_START    ; вышли за конец — заворот в начало
bn_ret: ret
END
