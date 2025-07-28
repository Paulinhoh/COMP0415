# Exemplo de Entrada:
# 10
# 5 13 -1 3 0 2 1 8 1 -2
#
# Exemplo de Saída:
# -2,-1,0,1,1,2,3,5,8,13

.data
comma:      .asciz ","

.bss
# Espaço para armazenar até 1000 números de 32-bit (1000 * 4 bytes = 4000 bytes)
number_array: .space 4000

.text
.globl main

# ==============================================================================
# FUNÇÃO PRINCIPAL
# ==============================================================================
main:
    # 1. Ler a quantidade de números (N) da primeira linha de entrada.
    jal ra, read_integer
    mv s0, a0               # Salva N em s0

    # 2. Ler N números e armazená-los no vetor. A função read_integer
    # irá pular a quebra de linha entre N e os números do vetor.
    la s1, number_array     # s1 = endereço base do vetor
    li t0, 0                # t0 = i = 0 (contador do loop)

read_loop:
    bge t0, s0, read_loop_end # if (i >= N) sai do loop
    jal ra, read_integer      # Lê o próximo número, resultado em a0
    sw a0, 0(s1)              # Salva o número no vetor: array[i] = a0
    addi s1, s1, 4            # Avança o ponteiro do vetor
    addi t0, t0, 1            # i++
    j read_loop

read_loop_end:

    # 3. Ordenar o vetor usando Bubble Sort
    la a0, number_array     # a0 = endereço do vetor
    mv a1, s0               # a1 = N (tamanho do vetor)
    jal ra, bubble_sort

    # 4. Imprimir o vetor ordenado
    la s1, number_array     # s1 = endereço base do vetor
    li t0, 0                # t0 = i = 0 (contador do loop)

print_loop:
    bge t0, s0, print_loop_end # if (i >= N) sai do loop
    lw a0, 0(s1)               # Carrega o número array[i] em a0 para impressão
    jal ra, print_integer      # Imprime o número

    # Imprimir vírgula, se não for o último elemento
    addi t1, t0, 1             # t1 = i + 1
    bge t1, s0, skip_comma     # if (i + 1 >= N), não imprime a vírgula
    
    la a0, comma               # Carrega o endereço da string ","
    li a7, 4                   # Syscall para imprimir string
    ecall

skip_comma:
    addi s1, s1, 4             # Avança o ponteiro do vetor
    addi t0, t0, 1             # i++
    j print_loop

print_loop_end:

    # 5. Finalizar o programa
    li a7, 10                  # Syscall para sair
    ecall

# ==============================================================================
# SUB-ROTINA: read_integer
# Lê um número inteiro com sinal da entrada padrão (formato ASCII).
# - Ignora espaços em branco iniciais, INCLUINDO QUEBRAS DE LINHA.
# - Trata números negativos.
# - Argumentos: Nenhum
# - Retorno: a0 = número lido
# ==============================================================================
read_integer:
    li t1, 0               # result = 0
    li t2, 1               # sign = 1 (positivo por padrão)

skip_whitespace:
    li a7, 12              # Syscall para ler um caractere
    ecall
    mv t0, a0              # Salva o caractere lido em t0
    
    # Esta seção é a chave para lidar com múltiplas linhas de entrada.
    # Ela trata espaço, nova linha e tab como separadores a serem ignorados.
    li t3, ' '
    beq t0, t3, skip_whitespace # Se for espaço, ignora
    li t3, '\n'
    beq t0, t3, skip_whitespace # Se for NOVA LINHA, ignora
    li t3, '\t'
    beq t0, t3, skip_whitespace # Se for tab, ignora

    # Verifica se há sinal de negativo
    li t3, '-'
    bne t0, t3, parse_digits
    
    li t2, -1              # sign = -1
    li a7, 12              # Lê o próximo caractere após o sinal
    ecall
    mv t0, a0

parse_digits:
    li t3, '0'
    blt t0, t3, end_read   # Se c < '0', fim do número
    li t3, '9'
    bgt t0, t3, end_read   # Se c > '9', fim do número

    li t3, '0'
    sub t0, t0, t3

    li t3, 10
    mul t1, t1, t3         # result = result * 10
    add t1, t1, t0         # result = result + digit

    li a7, 12
    ecall
    mv t0, a0
    j parse_digits

end_read:
    mul a0, t1, t2
    ret

# ==============================================================================
# SUB-ROTINA: print_integer
# (Esta rotina permanece idêntica à versão anterior)
# ==============================================================================
print_integer:
    mv t0, a0
    li t3, 0
    bne t0, zero, check_negative
    li a0, '0'
    li a7, 11
    ecall
    ret
check_negative:
    bgez t0, conversion_loop
    li a0, '-'
    li a7, 11
    ecall
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
    li a7, 11
    ecall
    addi t3, t3, -1
    bnez t3, print_stack_loop
    ret

# ==============================================================================
# SUB-ROTINA: bubble_sort
# (Esta rotina permanece idêntica à versão anterior)
# ==============================================================================
bubble_sort:
    mv s2, a0
    mv s3, a1
    li t0, 0
outer_loop:
    add t5, t0, 1
    bge t5, s3, end_sort
    li t1, 0
inner_loop:
    sub t5, s3, t0
    addi t5, t5, -1
    bge t1, t5, end_inner_loop
    slli t2, t1, 2
    add t2, s2, t2
    lw t3, 0(t2)
    lw t4, 4(t2)
    ble t3, t4, no_swap
    sw t4, 0(t2)
    sw t3, 4(t2)
no_swap:
    addi t1, t1, 1
    j inner_loop
end_inner_loop:
    addi t0, t0, 1
    j outer_loop
end_sort:
    ret