; Эталон чекера, А-12-23, 14, v1
$INCLUDE (vars.inc)
DSEG AT 30h
CSEG AT 0h
        sjmp START
org 03h                         ; INT0 — нажатие клавиши
        call KbScan ; %proc%
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
        setb PIN_CSEN           ; дешифратор CS включён постоянно
        mov DPTR, #ADR_KB
        clr A
        movx @DPTR, A           ; все столбцы в 0: любое нажатие опустит строку и INT0
        setb IT0                ; INT0 по спаду
        setb EX0
        setb EA
        jmp $ ; %stop%

; KbScan — сканирование матрицы по столбцам (обработчик INT0).
; Вход: нет. Выход: A — код единственной нажатой клавиши (табл. 3 ТЗ) или 0FFh,
; если не нажато ничего или нажато больше одной клавиши.
; Портит: B, R2–R6, DPTR, PSW. Столбцы по выходу снова все в 0.
KbScan:
        mov R4, #0FFh           ; код найденной клавиши, пока нет
        mov R2, #0              ; номер столбца
        mov R3, #0FEh           ; маска столбцов: 0 — в опрашиваемом
        mov DPTR, #ADR_KB
ks_col: mov A, R3
        movx @DPTR, A           ; опустить один столбец
        movx A, @DPTR           ; строки: 0 — замкнута на этот столбец
        anl A, #(1 SHL KB_ROWS) - 1
        xrl A, #(1 SHL KB_ROWS) - 1 ; теперь 1 — замкнутая строка
        jz ks_next
        mov R5, A
        dec A
        anl A, R5               ; ненулевой — в столбце больше одной строки
        jnz ks_many
        cjne R4, #0FFh, ks_many ; клавиша уже найдена в другом столбце
        mov A, R5
        mov R6, #0FFh
ks_bit: inc R6                  ; R6 — номер замкнутой строки
        rrc A
        jnc ks_bit
        mov A, R6
        mov B, #KB_COLS
        mul AB
        add A, R2               ; индекс = строка * KB_COLS + столбец
        push DPL
        push DPH
        mov DPTR, #KeyTab3 + (KB_COLS - 3) * 12
        movc A, @A+DPTR
        mov R4, A
        pop DPH
        pop DPL
ks_next:
        inc R2
        mov A, R3
        rl A
        mov R3, A
        cjne R2, #KB_COLS, ks_col
        sjmp ks_done
ks_many:
        mov R4, #0FFh
ks_done:
        clr A
        movx @DPTR, A           ; столбцы снова в 0 — ждать следующее нажатие
        clr IE0                 ; спады INT0 во время опроса — не новое нажатие
        mov A, R4
        ret

; Коды клавиш по рис. 4 ТЗ, [строка][столбец]: 3×4 (нечётный M) и 4×3 (чётный M)
KeyTab3: DB 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 10, 11
KeyTab4: DB 1, 2, 3, 10, 4, 5, 6, 11, 7, 8, 9, 0
END
