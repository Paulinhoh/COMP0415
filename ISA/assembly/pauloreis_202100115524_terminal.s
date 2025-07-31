# Descrição:
# Versão do programa de ordenação que utiliza o algoritmo Quicksort.
# A E/S de dados continua sendo feita byte a byte via UART (MMIO).

.bss
# Espaço para armazenar até 1000 números de 32-bit (1000 * 4 bytes = 4000 bytes)
number_array: .space 4000

.text
.globl main

# Endereços de memória para a UART (MMIO)
.eqv UART_RECEIVER, 0xffff0000
.eqv UART_TRANSMITTER, 0xffff0008

# ==============================================================================
# FUNÇÃO PRINCIPAL
# ==============================================================================
main:
    # 1. Ler a quantidade de números (N).
    jal ra, read_integer
    mv s0, a0               # Salva N em s0

    # 2. Ler N números e armazená-los no vetor.
    la s1, number_array
    li t0, 0
read_loop:
    bge t0, s0, read_loop_end
    jal ra, read_integer
    sw a0, 0(s1)
    addi s1, s1, 4
    addi t0, t0, 1
    j read_loop
read_loop_end:

    # 3. Ordenar o vetor usando Quicksort
    # Prepara os argumentos para quicksort(low_ptr, high_ptr)
    la a0, number_array      # a0 = ponteiro para o primeiro elemento (low)
    
    # Calcula o ponteiro para o último elemento (high)
    # high_ptr = base + (N-1) * 4
    addi t0, s0, -1          # t0 = N - 1
    slli t0, t0, 2           # t0 = (N-1) * 4 (offset em bytes)
    add a1, a0, t0           # a1 = ponteiro para o último elemento
    
    jal ra, quicksort

    # 4. Imprimir o vetor ordenado
    la s1, number_array
    li t0, 0
print_loop:
    bge t0, s0, print_loop_end
    lw a0, 0(s1)
    jal ra, print_integer

    addi t1, t0, 1
    bge t1, s0, skip_comma
    
    li a0, ','
    jal ra, uart_print_byte

skip_comma:
    addi s1, s1, 4
    addi t0, t0, 1
    j print_loop
print_loop_end:

    # 5. Finalizar o programa
    li a7, 10
    ecall

# ==============================================================================
# SUB-ROTINAS DE ORDENAÇÃO QUICKSORT
# ==============================================================================

# ------------------------------------------------------------------------------
# quicksort: Função recursiva que ordena um sub-array.
# - Argumentos: a0 = ponteiro 'low', a1 = ponteiro 'high'
# ------------------------------------------------------------------------------
quicksort:
    # Caso base: se low >= high, o sub-array tem 0 ou 1 elemento, já está ordenado.
    bge a0, a1, end_quicksort

    # Cria um frame na pilha para salvar o estado para a recursão
    addi sp, sp, -12
    sw ra, 8(sp)    # Salva o endereço de retorno
    sw s0, 4(sp)    # Usado para salvar o ponteiro 'high'
    sw s1, 0(sp)    # Usado para salvar o ponteiro do pivô

    mv s0, a1       # s0 = high
    
    # Chama partition. a0 e a1 já são os argumentos corretos (low, high).
    jal ra, partition
    # partition retorna o ponteiro do pivô em a0
    mv s1, a0       # s1 = pivot_ptr

    # Primeira chamada recursiva: quicksort(low, pivot_ptr - 4)
    # a0 (low) já está correto.
    addi a1, s1, -4
    jal ra, quicksort

    # Segunda chamada recursiva: quicksort(pivot_ptr + 4, high)
    addi a0, s1, 4
    mv a1, s0       # Restaura 'high' de s0
    jal ra, quicksort
    
    # Restaura o estado da pilha e retorna
    lw ra, 8(sp)
    lw s0, 4(sp)
    lw s1, 0(sp)
    addi sp, sp, 12

end_quicksort:
    ret

# ------------------------------------------------------------------------------
# partition: Rearranja o sub-array usando o esquema de Lomuto.
# - Argumentos: a0 = ponteiro 'low', a1 = ponteiro 'high'
# - Retorno: a0 = ponteiro para a posição final do pivô
# ------------------------------------------------------------------------------
partition:
    # s2 = valor do pivô, s3 = ponteiro i, s4 = ponteiro j
    lw s2, 0(a1)        # s2 = pivot_value = *(high_ptr)
    mv s3, a0           # s3 = i_ptr = low_ptr
    mv s4, a0           # s4 = j_ptr = low_ptr

partition_loop:
    bge s4, a1, end_partition_loop # Loop enquanto j_ptr < high_ptr
    
    lw t0, 0(s4)        # t0 = *j
    # Se *j > pivot_value, não faz a troca
    bgt t0, s2, no_swap 
    
    # Se *j <= pivot_value, troca *i com *j
    lw t1, 0(s3)        # t1 = *i
    sw t0, 0(s3)        # *i = *j
    sw t1, 0(s4)        # *j = t1
    addi s3, s3, 4      # i_ptr++
    
no_swap:
    addi s4, s4, 4      # j_ptr++
    j partition_loop

end_partition_loop:
    # Troca final para posicionar o pivô em seu lugar correto (*i)
    lw t0, 0(s3)        # t0 = *i
    sw t0, 0(a1)        # *high = *i
    sw s2, 0(s3)        # *i = pivot_value

    mv a0, s3           # Retorna o ponteiro do pivô (i_ptr)
    ret

# ==============================================================================
# SUB-ROTINAS DE E/S VIA UART (INALTERADAS)
# ==============================================================================
uart_read_byte:
    li t0, UART_RECEIVER
    lb a0, 0(t0)
    ret
uart_print_byte:
    li t0, UART_TRANSMITTER
    sb a0, 0(t0)
    ret
# ==============================================================================
# SUB-ROTINAS DE CONVERSÃO E IMPRESSÃO (INALTERADAS)
# ==============================================================================
read_integer:
    li t1, 0
    li t2, 1
skip_whitespace:
    jal ra, uart_read_byte
    mv t0, a0
    li t3, ' '
    beq t0, t3, skip_whitespace
    li t3, '\n'
    beq t0, t3, skip_whitespace
    li t3, '\t'
    beq t0, t3, skip_whitespace
    li t3, '-'
    bne t0, t3, parse_digits
    li t2, -1
    jal ra, uart_read_byte
    mv t0, a0
parse_digits:
    li t3, '0'
    blt t0, t3, end_read
    li t3, '9'
    bgt t0, t3, end_read
    li t3, '0'
    sub t0, t0, t3
    li t3, 10
    mul t1, t1, t3
    add t1, t1, t0
    jal ra, uart_read_byte
    mv t0, a0
    j parse_digits
end_read:
    mul a0, t1, t2
    ret

print_integer:
    mv t0, a0
    li t3, 0
    bne t0, zero, check_negative
    li a0, '0'
    jal ra, uart_print_byte
    ret
check_negative:
    bgez t0, conversion_loop
    li a0, '-'
    jal ra, uart_print_byte
    neg t0, t0
conversion_loop:
    li t1, 10
    rem t2, t0, t1
    div t0, t0, t1
    addi t2, t2, '0'
    addi sp, sp, -4
    sw t2, 0(sp)
    addi t3, t3, 1
    bnez t0, conversion_loop
print_stack_loop:
    lw t4, 0(sp)
    addi sp, sp, 4
    mv a0, t4
    jal ra, uart_print_byte
    addi t3, t3, -1
    bnez t3, print_stack_loop
    ret
