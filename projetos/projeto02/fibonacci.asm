# Paulo Henrique dos Santos Reis - 202100115524

# 01 - Mostra a sequencia de Fibonacci de acondo com o tamanho da sequencia que o usuario quer ver
# 02 - O que eu vejo que podia ter melhorado seria a implementação com recursão, mas eu fiquei um bom tempo batendo a cabeça 
# tentando fazer funcionar, num futuro tentarei novamente a recursão

.data
	msg: .asciiz "Digite o tamanho da sequência de Fibonacci: "
	space: .asciiz " "

.text
	# printa a msg
	li $v0, 4
	la $a0, msg
	syscall
	
	# guardar o valor em $v0
	li $v0, 5
	syscall
	
	move $t0, $v0 # move o valor de $v0 para $t0
	
	jal fibonacci # chama a função fibonacci
	
	# função fibonacci
	fibonacci:
		addi $t1, $zero, 0 # add valor de $t1
		addi $t2, $zero, 1 # add valor de $t2
	
		# printa $t1
		li $v0, 1
		move $a0, $t1
		syscall
		
		jal printSpace # chama a funçao printSpace
		
		# printa $t2
		li $v0, 1
		move $a0, $t2
		syscall
		
		addi $t4, $zero, 2 # contador
		# loop
		loop:
			beq $t4, $t0, endLoop # condicional
			
			jal printSpace
			
			add $t3, $t1, $t2 # soma t1 e t2 em t3
			
			# printa $t3
			li $v0, 1
			move $a0, $t3
			syscall
			
			move $t1, $t2 # move t2 para t1
			move $t2, $t3 # move t3 para t2 
			
			addi $t4, $t4, 1 # incrementa +1 ao contador
			j loop 
			
		endLoop:
			jal exit # chama a função de encerramento
	
	# função para printar o espaço entre os valores
	printSpace:
		li $v0, 4
		la $a0, space
		syscall
		jr $ra	
	
	#função de encerramento do programa
	exit:
		li $v0, 10
		syscall
