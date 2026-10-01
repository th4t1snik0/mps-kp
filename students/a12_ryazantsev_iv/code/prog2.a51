; Рязанцев И.В., А-12-23, 17, v1
$INCLUDE (vars.inc)             ; адреса, биты, константы варианта; в файл для робота mpscode вклеит его текст
DSEG AT 30h
; переменные не нужны: указатели и флаги буфера заданы в vars.inc
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
        setb PIN_CSEN           ; дешифратор CS включён — буфер доступен через movx
        mov HEAD_L, #LOW(BUF_START)     ; голова — на первую ячейку буфера
        mov HEAD_H, #HIGH(BUF_START)
        mov TAIL_L, #LOW(BUF_START)     ; хвост там же: читать нечего
        mov TAIL_H, #HIGH(BUF_START)
        setb F_EMPTY            ; буфер пуст
        clr F_OVF               ; переполнений ещё не было
        mov A, #55h             ; пример отсчёта
        call BufWrite ; %proc%
        jmp $ ; %stop%

; BufWrite — запись отсчёта в кольцевой буфер IDT7005 в ячейку «головы».
; Полон (голова догнала хвост, а буфер не пуст) — самый старый отсчёт выбрасывается
; (хвост сдвигается) и ставится F_OVF; вмещает все BUF_SIZE отсчётов.
; Вход: A — отсчёт. Выход: отсчёт в буфере, голова сдвинута, F_EMPTY = 0, F_OVF = 1 при затирании.
; Портит: A, флаги (DPTR сохраняется).
BufWrite:
        push DPL                ; DPTR нужен для movx, вернём вызывающему
        push DPH
        push ACC                ; отсчёт пригодится после проверок
        jb F_EMPTY, BwPut       ; пустой буфер переполниться не может
        mov A, HEAD_L           ; голова совпала с хвостом? — сравниваем оба байта
        cjne A, TAIL_L, BwPut
        mov A, HEAD_H
        cjne A, TAIL_H, BwPut
        mov DPL, TAIL_L         ; буфер полон: хвост — на следующий отсчёт,
        mov DPH, TAIL_H         ; самый старый будет затёрт
        call BufNext
        mov TAIL_L, DPL
        mov TAIL_H, DPH
        setb F_OVF              ; отметить потерю отсчёта
BwPut:
        mov DPL, HEAD_L         ; адрес ячейки «головы»
        mov DPH, HEAD_H
        pop ACC                 ; вернуть отсчёт
        movx @DPTR, A           ; записать его левым портом IDT7005
        call BufNext            ; голова — на следующую ячейку
        mov HEAD_L, DPL
        mov HEAD_H, DPH
        clr F_EMPTY             ; теперь в буфере точно что-то есть
        pop DPH
        pop DPL
        ret

; BufNext — следующая ячейка кольца: DPTR + 1, после BUF_END - 1 — снова BUF_START.
; Вход: DPTR — адрес ячейки буфера. Выход: DPTR — адрес следующей ячейки.
; Портит: A, флаги.
BufNext:
        inc DPTR
        mov A, DPL              ; дошли до первой ячейки за буфером?
        cjne A, #LOW(BUF_END), BnExit
        mov A, DPH
        cjne A, #HIGH(BUF_END), BnExit
        mov DPTR, #BUF_START    ; заворот на начало кольца
BnExit:
        ret
END
