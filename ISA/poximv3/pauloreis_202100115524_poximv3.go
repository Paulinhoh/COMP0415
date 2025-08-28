package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Constantes para a cache
const (
	CACHE_SIZE      = 256    // 256 bytes
	BLOCK_SIZE      = 16     // 4 palavras de 4 bytes = 16 bytes
	ASSOCIATIVITY   = 2      // Grau de associatividade
	NUM_SETS        = CACHE_SIZE / (BLOCK_SIZE * ASSOCIATIVITY)
	BLOCK_WORDS     = BLOCK_SIZE / 4
	OFFSET_BITS     = 4      // log2(BLOCK_SIZE)
	INDEX_BITS      = 3      // log2(NUM_SETS)
	TAG_BITS        = 32 - INDEX_BITS - OFFSET_BITS
)

// Estrutura da linha de cache
type CacheLine struct {
	valid   [ASSOCIATIVITY]bool
	tag     [ASSOCIATIVITY]uint32
	age     [ASSOCIATIVITY]uint32 // Para política LRU
	data    [ASSOCIATIVITY][BLOCK_WORDS]uint32
}

// Estrutura da cache
type Cache struct {
	sets     [NUM_SETS]CacheLine
	hits     int
	misses   int
	accesses int
}

// Variáveis globais para as caches
var icache Cache // Cache de instruções
var dcache Cache // Cache de dados

// Inicializar cache
func initCache(cache *Cache) {
	for i := 0; i < NUM_SETS; i++ {
		for j := 0; j < ASSOCIATIVITY; j++ {
			cache.sets[i].valid[j] = false
			cache.sets[i].tag[j] = 0
			cache.sets[i].age[j] = 0
		}
	}
	cache.hits = 0
	cache.misses = 0
	cache.accesses = 0
}

// Extrair tag, índice e offset do endereço
func extractAddressFields(address uint32) (uint32, uint32, uint32) {
	tag := address >> (INDEX_BITS + OFFSET_BITS)
	index := (address >> OFFSET_BITS) & ((1 << INDEX_BITS) - 1)
	offset := (address & ((1 << OFFSET_BITS) - 1)) >> 2 // Deslocamento em palavras
	return tag, index, offset
}

// Acessar cache de instruções
func accessICache(address uint32, writer *bufio.Writer) (uint32, bool) {
	tag, index, offset := extractAddressFields(address)
	cache := &icache
	cache.accesses++

	// Verificar se está na cache
	for i := 0; i < ASSOCIATIVITY; i++ {
		if cache.sets[index].valid[i] && cache.sets[index].tag[i] == tag {
			// Hit - atualizar idade LRU
			cache.hits++
			for j := 0; j < ASSOCIATIVITY; j++ {
				if cache.sets[index].age[j] > cache.sets[index].age[i] {
					cache.sets[index].age[j]--
				}
			}
			cache.sets[index].age[i] = uint32(ASSOCIATIVITY - 1)
			
			// Log de hit
			fmt.Fprintf(writer, "#cache_mem:irh 0x%08x    line=%d, age=%d, id=0x%06x, block[%d]={0x%08x, 0x%08x, 0x%08x, 0x%08x}\n",
				address, index, cache.sets[index].age[i], tag, offset,
				cache.sets[index].data[i][0], cache.sets[index].data[i][1],
				cache.sets[index].data[i][2], cache.sets[index].data[i][3])
			
			return cache.sets[index].data[i][offset], true
		}
	}

	// Miss
	cache.misses++
	
	// Log de miss
	validStr := fmt.Sprintf("{%t,%t}", cache.sets[index].valid[0], cache.sets[index].valid[1])
	ageStr := fmt.Sprintf("{%d,%d}", cache.sets[index].age[0], cache.sets[index].age[1])
	idStr := fmt.Sprintf("{0x%06x,0x%06x}", cache.sets[index].tag[0], cache.sets[index].tag[1])
	fmt.Fprintf(writer, "#cache_mem:irm 0x%08x    line=%d, valid=%s, age=%s, id=%s\n",
		address, index, validStr, ageStr, idStr)
	
	return 0, false
}

// Acessar cache de dados (leitura)
func accessDCacheRead(address uint32, writer *bufio.Writer) (uint32, bool) {
	tag, index, offset := extractAddressFields(address)
	cache := &dcache
	cache.accesses++

	// Verificar se está na cache
	for i := 0; i < ASSOCIATIVITY; i++ {
		if cache.sets[index].valid[i] && cache.sets[index].tag[i] == tag {
			// Hit - atualizar idade LRU
			cache.hits++
			for j := 0; j < ASSOCIATIVITY; j++ {
				if cache.sets[index].age[j] > cache.sets[index].age[i] {
					cache.sets[index].age[j]--
				}
			}
			cache.sets[index].age[i] = uint32(ASSOCIATIVITY - 1)
			
			// Log de hit
			fmt.Fprintf(writer, "#cache_mem:drh 0x%08x    line=%d, age=%d, id=0x%06x, block[%d]={0x%08x, 0x%08x, 0x%08x, 0x%08x}\n",
				address, index, cache.sets[index].age[i], tag, offset,
				cache.sets[index].data[i][0], cache.sets[index].data[i][1],
				cache.sets[index].data[i][2], cache.sets[index].data[i][3])
			
			return cache.sets[index].data[i][offset], true
		}
	}

	// Miss - escrita direta sem alocação (no write allocate)
	cache.misses++
	
	// Log de miss
	validStr := fmt.Sprintf("{%t,%t}", cache.sets[index].valid[0], cache.sets[index].valid[1])
	ageStr := fmt.Sprintf("{%d,%d}", cache.sets[index].age[0], cache.sets[index].age[1])
	idStr := fmt.Sprintf("{0x%06x,0x%06x}", cache.sets[index].tag[0], cache.sets[index].tag[1])
	fmt.Fprintf(writer, "#cache_mem:drm 0x%08x    line=%d, valid=%s, age=%s, id=%s\n",
		address, index, validStr, ageStr, idStr)
	
	return 0, false
}

// Acessar cache de dados (escrita)
func accessDCacheWrite(address uint32, value uint32, writer *bufio.Writer) {
	tag, index, offset := extractAddressFields(address)
	cache := &dcache
	cache.accesses++

	// Verificar se está na cache
	found := false
	for i := 0; i < ASSOCIATIVITY; i++ {
		if cache.sets[index].valid[i] && cache.sets[index].tag[i] == tag {
			// Hit - atualizar dado e idade LRU
			cache.hits++
			cache.sets[index].data[i][offset] = value
			for j := 0; j < ASSOCIATIVITY; j++ {
				if cache.sets[index].age[j] > cache.sets[index].age[i] {
					cache.sets[index].age[j]--
				}
			}
			cache.sets[index].age[i] = uint32(ASSOCIATIVITY - 1)
			found = true
			
			// Log de hit
			fmt.Fprintf(writer, "#cache_mem:dwh 0x%08x    line=%d, age=%d, id=0x%06x, block[%d]={0x%08x, 0x%08x, 0x%08x, 0x%08x}\n",
				address, index, cache.sets[index].age[i], tag, offset,
				cache.sets[index].data[i][0], cache.sets[index].data[i][1],
				cache.sets[index].data[i][2], cache.sets[index].data[i][3])
			break
		}
	}

	if !found {
		// Miss - escrita direta sem alocação (no write allocate)
		cache.misses++
		
		// Log de miss
		validStr := fmt.Sprintf("{%t,%t}", cache.sets[index].valid[0], cache.sets[index].valid[1])
		ageStr := fmt.Sprintf("{%d,%d}", cache.sets[index].age[0], cache.sets[index].age[1])
		idStr := fmt.Sprintf("{0x%06x,0x%06x}", cache.sets[index].tag[0], cache.sets[index].tag[1])
		fmt.Fprintf(writer, "#cache_mem:dwm 0x%08x    line=%d, valid=%s, age=%s, id=%s\n",
			address, index, validStr, ageStr, idStr)
	}
}

// Carregar bloco na cache
func loadBlockToCache(cache *Cache, address uint32, mem []byte, offset uint32, isInstruction bool) {
	tag, index, _ := extractAddressFields(address)
	blockAddr := address & ^uint32(BLOCK_SIZE-1)
	
	// Encontrar vítima usando LRU
	victim := 0
	for i := 1; i < ASSOCIATIVITY; i++ {
		if cache.sets[index].age[i] < cache.sets[index].age[victim] || !cache.sets[index].valid[i] {
			victim = i
		}
	}
	
	// Carregar bloco da memória
	for i := 0; i < BLOCK_WORDS; i++ {
		wordAddr := blockAddr + uint32(i*4)
		if wordAddr >= offset && wordAddr < offset+uint32(len(mem))-3 {
			idxMem := wordAddr - offset
			cache.sets[index].data[victim][i] = binary.LittleEndian.Uint32(mem[idxMem : idxMem+4])
		}
	}
	
	// Atualizar metadados
	cache.sets[index].valid[victim] = true
	cache.sets[index].tag[victim] = tag
	
	// Atualizar idades LRU
	for i := 0; i < ASSOCIATIVITY; i++ {
		if i != victim && cache.sets[index].valid[i] {
			if cache.sets[index].age[i] > 0 {
				cache.sets[index].age[i]--
			}
		}
	}
	cache.sets[index].age[victim] = uint32(ASSOCIATIVITY - 1)
}

func carregarMemoria(caminhoArquivo string, mem []byte, offset uint32) {
	arquivo, err := os.Open(caminhoArquivo)
	if err != nil {
		log.Fatalf("Falha ao abrir o arquivo de entrada: %v", err)
	}
	defer arquivo.Close()

	scanner := bufio.NewScanner(arquivo)
	var endereco uint32 = 0
	for scanner.Scan() {
		linha := strings.TrimSpace(scanner.Text())
		if linha == "" {
			continue
		}

		if strings.HasPrefix(linha, "@") {
			addr, err := strconv.ParseUint(linha[1:], 16, 32)
			if err != nil {
				log.Fatalf("Endereço inválido: %s", linha)
			}
			endereco = uint32(addr)
		} else {
			stringsDeBytes := strings.Fields(linha)
			for _, stringDoByte := range stringsDeBytes {
				valorDoByte, err := strconv.ParseUint(stringDoByte, 16, 8)
				if err != nil {
					log.Fatalf("Byte inválido: %s", stringDoByte)
				}
				idxMem := endereco - offset
				if idxMem < uint32(len(mem)) {
					mem[idxMem] = byte(valorDoByte)
				}
				endereco++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Erro ao ler o arquivo de entrada: %v", err)
	}
}

func lerInstrucao(mem []byte, pc, offset uint32, writer *bufio.Writer) (uint32, bool) {
	if pc < offset || pc+3 >= offset+uint32(len(mem)) {
		return 0, false
	}
	
	// Acessar cache de instruções
	instrucao, hit := accessICache(pc, writer)
	if hit {
		return instrucao, true
	}
	
	// Cache miss - carregar bloco
	loadBlockToCache(&icache, pc, mem, offset, true)
	
	// Tentar novamente
	instrucao, hit = accessICache(pc, writer)
	if hit {
		return instrucao, true
	}
	
	// Fallback: acesso direto à memória
	idxMem := pc - offset
	return binary.LittleEndian.Uint32(mem[idxMem : idxMem+4]), true
}

func estenderSinal(valor uint32, bits uint) int32 {
	desloca := 32 - bits
	return int32(valor<<desloca) >> desloca
}

// Constantes para os endereços dos CSRs
const (
	MSTATUS = 0x300
	MIE     = 0x304
	MTVEC   = 0x305
	MEPC    = 0x341
	MCAUSE  = 0x342
	MTVAL   = 0x343
	MIP     = 0x344
)

// Constantes para os bits dos CSRs
const (
	MSTATUS_MIE_BIT  = 1 << 3
	MSTATUS_MPIE_BIT = 1 << 7
	MIP_MTIP_BIT     = 1 << 7
	MIP_MSIP_BIT     = 1 << 3
	MIP_MEIP_BIT     = 1 << 11
)

// Constantes para os códigos de exceção
const (
	EXC_INSTRUCTION_ACCESS_FAULT = 1
	EXC_ILLEGAL_INSTRUCTION      = 2
	EXC_LOAD_ACCESS_FAULT        = 5
	EXC_STORE_ACCESS_FAULT       = 7
	EXC_ECALL_FROM_M_MODE        = 11
)

// Constantes para os códigos de interrupção
const (
	INT_MACHINE_SOFTWARE = 3
	INT_MACHINE_TIMER    = 7
	INT_MACHINE_EXTERNAL = 11
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Uso: %s <arquivo_entrada> <arquivo_saida>", os.Args[0])
	}
	caminhoArquivoEntrada := os.Args[1]
	caminhoArquivoSaida := os.Args[2]

	arquivoSaida, err := os.Create(caminhoArquivoSaida)
	if err != nil {
		log.Fatalf("Falha ao criar o arquivo de saída: %v", err)
	}
	defer arquivoSaida.Close()
	writer := bufio.NewWriter(arquivoSaida)
	defer writer.Flush()

	const offset uint32 = 0x80000000
	const tamMem = 32 * 1024

	x := make([]int32, 32)
	xLabel := []string{
		"zero", "ra", "sp", "gp", "tp", "t0", "t1", "t2", "s0", "s1",
		"a0", "a1", "a2", "a3", "a4", "a5", "a6", "a7", "s2", "s3",
		"s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "t3", "t4",
		"t5", "t6",
	}

	pc := offset
	mem := make([]byte, tamMem)

	// Inicializar caches
	initCache(&icache)
	initCache(&dcache)

	// Mapa de nomes de exceções
	exceptionNames := map[uint32]string{
		EXC_INSTRUCTION_ACCESS_FAULT: "instruction_fault",
		EXC_ILLEGAL_INSTRUCTION:      "illegal_instruction",
		EXC_LOAD_ACCESS_FAULT:        "load_fault",
		EXC_STORE_ACCESS_FAULT:       "store_fault",
		EXC_ECALL_FROM_M_MODE:        "environment_call",
	}
	// Mapa de nomes de interrupções
	interruptNames := map[uint32]string{
		INT_MACHINE_SOFTWARE: "software",
		INT_MACHINE_TIMER:    "timer",
		INT_MACHINE_EXTERNAL: "external",
	}

	csr := make(map[uint32]uint32)
	csr[MSTATUS] = 0
	csr[MTVEC] = 0
	csr[MIE] = 0
	csr[MIP] = 0 // Inicializa o Machine Interrupt Pending

	carregarMemoria(caminhoArquivoEntrada, mem, offset)

	gerarExcecao := func(codigoTrap, valorTrap uint32, isInterrupt bool) {
		// Salva o PC atual e define a causa
		csr[MEPC] = pc
		csr[MTVAL] = valorTrap

		if isInterrupt {
			csr[MCAUSE] = (1 << 31) | codigoTrap // Bit 31 setado para interrupções
		} else {
			csr[MCAUSE] = codigoTrap
		}

		// Desabilita interrupções globais e salva o estado anterior
		if (csr[MSTATUS] & MSTATUS_MIE_BIT) != 0 {
			csr[MSTATUS] |= MSTATUS_MPIE_BIT // Salva MIE em MPIE
		} else {
			csr[MSTATUS] &^= MSTATUS_MPIE_BIT
		}
		csr[MSTATUS] &^= MSTATUS_MIE_BIT // Desabilita MIE

		var eventName string
		var eventType string

		if isInterrupt {
			eventType = "interrupt"
			var ok bool
			eventName, ok = interruptNames[codigoTrap]
			if !ok {
				eventName = "Unknown Interrupt"
			}
		} else {
			eventType = "exception"
			var ok bool
			eventName, ok = exceptionNames[codigoTrap]
			if !ok {
				eventName = "Unknown Exception"
			}
		}

		fmt.Fprintf(writer, ">%s:%s 			cause=0x%08x,epc=0x%08x,tval=0x%08x\n", eventType, eventName, csr[MCAUSE], csr[MEPC], csr[MTVAL])

		// Pula para o endereço do tratador de trap
		pc = csr[MTVEC] & ^uint32(0x3) // Modo direto
	}

	executando := true
	for executando {
		x[0] = 0

		// Verifica se há interrupções habilitadas e pendentes
		mieGlobal := (csr[MSTATUS] & MSTATUS_MIE_BIT) != 0
		interrupcoesPendentes := csr[MIE] & csr[MIP]

		if mieGlobal && interrupcoesPendentes != 0 {
			var interruptCode uint32

			// Prioridade: Externa > Software > Timer
			if (interrupcoesPendentes & MIP_MEIP_BIT) != 0 {
				interruptCode = INT_MACHINE_EXTERNAL
			} else if (interrupcoesPendentes & MIP_MSIP_BIT) != 0 {
				interruptCode = INT_MACHINE_SOFTWARE
			} else if (interrupcoesPendentes & MIP_MTIP_BIT) != 0 {
				interruptCode = INT_MACHINE_TIMER
			}

			if interruptCode != 0 {
				gerarExcecao(interruptCode, 0, true)
				// Limpa o bit da interrupção pendente após tratá-la (Exemplo para timer)
				if interruptCode == INT_MACHINE_TIMER {
					csr[MIP] &^= MIP_MTIP_BIT
				}
				continue // Reinicia o loop para o PC do tratador
			}
		}

		instrucao, ok := lerInstrucao(mem, pc, offset, writer)
		if !ok {
			gerarExcecao(EXC_INSTRUCTION_ACCESS_FAULT, pc, false)
			continue
		}

		proximoPC := pc + 4

		opcode := instrucao & 0x7F
		rd := (instrucao >> 7) & 0x1F
		rs1 := (instrucao >> 15) & 0x1F
		rs2 := (instrucao >> 20) & 0x1F
		funct3 := (instrucao >> 12) & 0x7
		funct7 := (instrucao >> 25) & 0x7F

		switch opcode {
		case 0b0110111: // lui
			immU := instrucao & 0xFFFFF000
			resultado := int32(immU)
			fmt.Fprintf(writer, "0x%08x:lui    %s,0x%05x   rd=0x%08x\n", pc, xLabel[rd], immU>>12, uint32(resultado))
			if rd != 0 {
				x[rd] = resultado
			}

		case 0b0010111: // auipc
			immU := instrucao & 0xFFFFF000
			resultado := int32(pc) + int32(immU)
			fmt.Fprintf(writer, "0x%08x:auipc  %s,0x%05x   rd=0x%08x+0x%08x=0x%08x\n", pc, xLabel[rd], immU>>12, pc, immU, uint32(resultado))
			if rd != 0 {
				x[rd] = resultado
			}

		case 0b0000011: // Load instructions
			immI := instrucao >> 20
			immSinalI := estenderSinal(immI, 12)
			enderecoMem := uint32(x[rs1]) + uint32(immSinalI)

			if enderecoMem < offset || enderecoMem >= offset+uint32(tamMem) {
				gerarExcecao(EXC_LOAD_ACCESS_FAULT, enderecoMem, false)
				proximoPC = pc
			} else {
				var data int32
				inst := ""
				idxMem := enderecoMem - offset
				
				// Acessar cache de dados para leitura
				dataUint, hit := accessDCacheRead(enderecoMem, writer)
				if !hit {
					// Cache miss - carregar bloco
					loadBlockToCache(&dcache, enderecoMem, mem, offset, false)
					
					// Tentar novamente
					dataUint, hit = accessDCacheRead(enderecoMem, writer)
					if !hit {
						// Fallback: acesso direto à memória
						switch funct3 {
						case 0b000: // lb
							data = int32(int8(mem[idxMem]))
						case 0b100: // lbu
							data = int32(mem[idxMem])
						case 0b001: // lh
							data = int32(int16(binary.LittleEndian.Uint16(mem[idxMem : idxMem+2])))
						case 0b101: // lhu
							data = int32(binary.LittleEndian.Uint16(mem[idxMem : idxMem+2]))
						case 0b010: // lw
							data = int32(binary.LittleEndian.Uint32(mem[idxMem : idxMem+4]))
						default:
							gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
							proximoPC = pc
							goto fimLoop
						}
					} else {
						data = int32(dataUint)
					}
				} else {
					data = int32(dataUint)
				}
				
				switch funct3 {
				case 0b000: // lb
					inst = "lb"
				case 0b100: // lbu
					inst = "lbu"
				case 0b001: // lh
					inst = "lh"
				case 0b101: // lhu
					inst = "lhu"
				case 0b010: // lw
					inst = "lw"
				default:
					gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
					proximoPC = pc
					goto fimLoop
				}

				fmt.Fprintf(writer, "0x%08x:%-7s%s,0x%03x(%s)   %s=mem[0x%08x]=0x%08x\n", pc, inst, xLabel[rd], immSinalI&0xFFF, xLabel[rs1], xLabel[rd], enderecoMem, uint32(data))
				if rd != 0 {
					x[rd] = data
				}
			}

		case 0b0100011: // Store instructions
			bitsImmS := ((instrucao>>25)&0x7F)<<5 | ((instrucao >> 7) & 0x1F)
			immSinalS := estenderSinal(bitsImmS, 12)
			enderecoMem := uint32(x[rs1]) + uint32(immSinalS)

			if enderecoMem < offset || enderecoMem >= offset+uint32(tamMem) {
				gerarExcecao(EXC_STORE_ACCESS_FAULT, enderecoMem, false)
				proximoPC = pc
			} else {
				inst := ""
				stringOperacao := ""
				idxMem := enderecoMem - offset
				
				// Acessar cache de dados para escrita
				accessDCacheWrite(enderecoMem, uint32(x[rs2]), writer)
				
				// Escrita direta (write through) - sempre escreve na memória também
				switch funct3 {
				case 0b000: // sb
					inst = "sb"
					val := byte(x[rs2])
					stringOperacao = fmt.Sprintf("0x%02x", val)
					mem[idxMem] = val
				case 0b001: // sh
					inst = "sh"
					val := uint16(x[rs2])
					stringOperacao = fmt.Sprintf("0x%04x", val)
					binary.LittleEndian.PutUint16(mem[idxMem:idxMem+2], val)
				case 0b010: // sw
					inst = "sw"
					val := uint32(x[rs2])
					stringOperacao = fmt.Sprintf("0x%08x", val)
					binary.LittleEndian.PutUint32(mem[idxMem:idxMem+4], val)
				default:
					gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
					proximoPC = pc
					goto fimLoop
				}
				fmt.Fprintf(writer, "0x%08x:%-7s%s,0x%03x(%s)   mem[0x%08x]=%s\n", pc, inst, xLabel[rs2], immSinalS&0xFFF, xLabel[rs1], enderecoMem, stringOperacao)
			}

		case 0b0110011: // R-type
			var data int32
			inst := ""
			stringOperacao := ""
			quantDeslocamento := uint32(x[rs2]) & 0x1F

			if funct7 == 0b0000001 {
				s1, s2 := x[rs1], x[rs2]
				u1, u2 := uint32(s1), uint32(s2)
				switch funct3 {
				case 0b000: // mul
					inst, stringOperacao = "mul", fmt.Sprintf("0x%08x*0x%08x", u1, u2)
					data = s1 * s2
				case 0b001: // mulh
					inst, stringOperacao = "mulh", fmt.Sprintf("(hi)0x%08x*0x%08x", u1, u2)
					data = int32((int64(s1) * int64(s2)) >> 32)
				case 0b010: // mulhsu
					inst, stringOperacao = "mulhsu", fmt.Sprintf("(hi)0x%08x*(U)0x%08x", u1, u2)
					data = int32((int64(s1) * int64(int64(u2)&0xFFFFFFFF)) >> 32)
				case 0b011: // mulhu
					inst, stringOperacao = "mulhu", fmt.Sprintf("(hi)(U)0x%08x*(U)0x%08x", u1, u2)
					data = int32((uint64(u1) * uint64(u2)) >> 32)
				case 0b100: // div
					inst, stringOperacao = "div", fmt.Sprintf("0x%08x/0x%08x", u1, u2)
					if s2 == 0 {
						data = -1
					} else if s1 == -2147483648 && s2 == -1 {
						data = s1
					} else {
						data = s1 / s2
					}
				case 0b101: // divu
					inst, stringOperacao = "divu", fmt.Sprintf("(U)0x%08x/(U)0x%08x", u1, u2)
					if u2 == 0 {
						data = -1
					} else {
						data = int32(u1 / u2)
					}
				case 0b110: // rem
					inst, stringOperacao = "rem", fmt.Sprintf("0x%08x%%0x%08x", u1, u2)
					if s2 == 0 {
						data = s1
					} else if s1 == -2147483648 && s2 == -1 {
						data = 0
					} else {
						data = s1 % s2
					}
				case 0b111: // remu
					inst, stringOperacao = "remu", fmt.Sprintf("(U)0x%08x%%(U)0x%08x", u1, u2)
					if u2 == 0 {
						data = int32(u1)
					} else {
						data = int32(u1 % u2)
					}
				}
			} else {
				switch funct3 {
				case 0b111: // and
					inst, stringOperacao = "and", fmt.Sprintf("0x%08x&0x%08x", uint32(x[rs1]), uint32(x[rs2]))
					data = x[rs1] & x[rs2]
				case 0b110: // or
					inst, stringOperacao = "or", fmt.Sprintf("0x%08x|0x%08x", uint32(x[rs1]), uint32(x[rs2]))
					data = x[rs1] | x[rs2]
				case 0b100: // xor
					inst, stringOperacao = "xor", fmt.Sprintf("0x%08x^0x%08x", uint32(x[rs1]), uint32(x[rs2]))
					data = x[rs1] ^ x[rs2]
				case 0b001: // sll
					inst, stringOperacao = "sll", fmt.Sprintf("0x%08x<<%d", uint32(x[rs1]), quantDeslocamento)
					data = x[rs1] << quantDeslocamento
				case 0b101:
					if funct7 == 0 { // srl
						inst, stringOperacao = "srl", fmt.Sprintf("0x%08x>>%d", uint32(x[rs1]), quantDeslocamento)
						data = int32(uint32(x[rs1]) >> quantDeslocamento)
					} else { // sra
						inst, stringOperacao = "sra", fmt.Sprintf("0x%08x>>%d", uint32(x[rs1]), quantDeslocamento)
						data = x[rs1] >> quantDeslocamento
					}
				case 0b010: // slt
					inst, stringOperacao = "slt", fmt.Sprintf("(0x%08x<0x%08x)", uint32(x[rs1]), uint32(x[rs2]))
					if x[rs1] < x[rs2] {
						data = 1
					} else {
						data = 0
					}
				case 0b011: // sltu
					inst, stringOperacao = "sltu", fmt.Sprintf("(0x%08x<0x%08x) (unsigned)", uint32(x[rs1]), uint32(x[rs2]))
					if uint32(x[rs1]) < uint32(x[rs2]) {
						data = 1
					} else {
						data = 0
					}
				case 0b000:
					if funct7 == 0 { // add
						inst, stringOperacao = "add", fmt.Sprintf("0x%08x+0x%08x", uint32(x[rs1]), uint32(x[rs2]))
						data = x[rs1] + x[rs2]
					} else { // sub
						inst, stringOperacao = "sub", fmt.Sprintf("0x%08x-0x%08x", uint32(x[rs1]), uint32(x[rs2]))
						data = x[rs1] - x[rs2]
					}
				}
			}
			fmt.Fprintf(writer, "0x%08x:%-7s%s,%s,%s   %s -> 0x%08x\n", pc, inst, xLabel[rd], xLabel[rs1], xLabel[rs2], stringOperacao, uint32(data))
			if rd != 0 {
				x[rd] = data
			}

		case 0b0010011: // I-type
			immI := instrucao >> 20
			immSinalI := estenderSinal(immI, 12)
			quantDeslocamento := (instrucao >> 20) & 0x1F

			var data int32
			inst := ""
			stringOperacao := ""

			switch funct3 {
			case 0b111: // andi
				inst, stringOperacao = "andi", fmt.Sprintf("0x%08x&0x%08x", uint32(x[rs1]), uint32(immSinalI))
				data = x[rs1] & immSinalI
			case 0b110: // ori
				inst, stringOperacao = "ori", fmt.Sprintf("0x%08x|0x%08x", uint32(x[rs1]), uint32(immSinalI))
				data = x[rs1] | immSinalI
			case 0b100: // xori
				inst, stringOperacao = "xori", fmt.Sprintf("0x%08x^0x%08x", uint32(x[rs1]), uint32(immSinalI))
				data = x[rs1] ^ immSinalI
			case 0b001: // slli
				inst, stringOperacao = "slli", fmt.Sprintf("0x%08x<<%d", uint32(x[rs1]), quantDeslocamento)
				data = x[rs1] << quantDeslocamento
			case 0b101:
				if (instrucao >> 30) == 0 { // srli
					inst, stringOperacao = "srli", fmt.Sprintf("0x%08x>>%d", uint32(x[rs1]), quantDeslocamento)
					data = int32(uint32(x[rs1]) >> quantDeslocamento)
				} else { // srai
					inst, stringOperacao = "srai", fmt.Sprintf("0x%08x>>%d", uint32(x[rs1]), quantDeslocamento)
					data = x[rs1] >> quantDeslocamento
				}
			case 0b010: // slti
				inst, stringOperacao = "slti", fmt.Sprintf("(0x%08x<%d)", uint32(x[rs1]), immSinalI)
				if x[rs1] < immSinalI {
					data = 1
				} else {
					data = 0
				}
			case 0b011: // sltiu
				inst, stringOperacao = "sltiu", fmt.Sprintf("(0x%08x<%d)", uint32(x[rs1]), immSinalI)
				if uint32(x[rs1]) < uint32(immSinalI) {
					data = 1
				} else {
					data = 0
				}
			case 0b000: // addi
				inst, stringOperacao = "addi", fmt.Sprintf("0x%08x+0x%08x", uint32(x[rs1]), uint32(immSinalI))
				data = x[rs1] + immSinalI
			default:
				gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
				proximoPC = pc
				goto fimLoop
			}

			imediatoStr := fmt.Sprintf("0x%03x", immSinalI&0xFFF)
			if funct3 == 0b001 || funct3 == 0b101 {
				imediatoStr = fmt.Sprintf("%d", quantDeslocamento)
			}
			fmt.Fprintf(writer, "0x%08x:%-7s%s,%s,%s   %s -> 0x%08x\n", pc, inst, xLabel[rd], xLabel[rs1], imediatoStr, stringOperacao, uint32(data))
			if rd != 0 {
				x[rd] = data
			}

		case 0b1100011: // B-type
			bitsImmB := ((instrucao >> 8) & 0xF) << 1
			bitsImmB |= ((instrucao >> 25) & 0x3F) << 5
			bitsImmB |= ((instrucao >> 7) & 1) << 11
			bitsImmB |= ((instrucao >> 31) & 1) << 12
			immSinalB := estenderSinal(bitsImmB, 13)

			desviar := false
			charOperacao := ""
			inst := ""

			switch funct3 {
			case 0b000: // beq
				inst, charOperacao = "beq", "=="
				if x[rs1] == x[rs2] {
					desviar = true
				}
			case 0b001: // bne
				inst, charOperacao = "bne", "!="
				if x[rs1] != x[rs2] {
					desviar = true
				}
			case 0b100: // blt
				inst, charOperacao = "blt", "<"
				if x[rs1] < x[rs2] {
					desviar = true
				}
			case 0b101: // bge
				inst, charOperacao = "bge", ">="
				if x[rs1] >= x[rs2] {
					desviar = true
				}
			case 0b110: // bltu
				inst, charOperacao = "bltu", "<(U)"
				if uint32(x[rs1]) < uint32(x[rs2]) {
					desviar = true
				}
			case 0b111: // bgeu
				inst, charOperacao = "bgeu", ">=(U)"
				if uint32(x[rs1]) >= uint32(x[rs2]) {
					desviar = true
				}
			default:
				gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
				proximoPC = pc
				goto fimLoop
			}

			resultadoComparacao := 0
			if desviar {
				resultadoComparacao = 1
			}

			pcAlvo := pc + uint32(immSinalB)
			pcDestino := proximoPC
			if desviar {
				pcDestino = pcAlvo
			}

			fmt.Fprintf(writer, "0x%08x:%-7s%s,%s,0x%08x   (0x%08x%s0x%08x)=%d->pc=0x%08x\n", pc, inst, xLabel[rs1], xLabel[rs2], pcAlvo, uint32(x[rs1]), charOperacao, uint32(x[rs2]), resultadoComparacao, pcDestino)

			if desviar {
				proximoPC = pcAlvo
			}

		case 0b1101111: // jal
			bitsImmJ := ((instrucao >> 21) & 0x3FF) << 1
			bitsImmJ |= ((instrucao >> 20) & 0x1) << 11
			bitsImmJ |= ((instrucao >> 12) & 0xFF) << 12
			bitsImmJ |= ((instrucao >> 31) & 1) << 20
			immSinalJ := estenderSinal(bitsImmJ, 21)

			valorRd := int32(proximoPC)
			pcAlvo := pc + uint32(immSinalJ)
			fmt.Fprintf(writer, "0x%08x:jal    %s,0x%08x   pc=0x%08x,rd=0x%08x\n", pc, xLabel[rd], pcAlvo, pcAlvo, uint32(valorRd))
			if rd != 0 {
				x[rd] = valorRd
			}
			proximoPC = pcAlvo

		case 0b1100111: // jalr
			immI := instrucao >> 20
			immSinalI := estenderSinal(immI, 12)

			valorRd := int32(proximoPC)
			enderecoAlvo := (uint32(x[rs1]) + uint32(immSinalI)) & ^uint32(1)
			fmt.Fprintf(writer, "0x%08x:jalr   %s,%s,0x%03x   pc=0x%08x+0x%08x,rd=0x%08x\n", pc, xLabel[rd], xLabel[rs1], immSinalI&0xFFF, uint32(x[rs1]), uint32(immSinalI), uint32(valorRd))
			if rd != 0 {
				x[rd] = valorRd
			}
			proximoPC = enderecoAlvo

		case 0b1110011: // SYSTEM
			csrAddr := (instrucao >> 20) & 0xFFF
			immU := (instrucao >> 15) & 0x1F

			switch funct3 {
			case 0b000:
				switch (instrucao >> 20) & 0xFFF {
				case 0b000000000000: // ecall
					gerarExcecao(EXC_ECALL_FROM_M_MODE, 0, false)
					proximoPC = pc
				case 0b000000000001: // ebreak
					fmt.Fprintf(writer, "0x%08x:ebreak\n", pc)
					executando = false
				case 0b001100000010: // mret
					fmt.Fprintf(writer, "0x%08x:mret\n", pc)
					// Restaura o estado de habilitação de interrupção
					if (csr[MSTATUS] & MSTATUS_MPIE_BIT) != 0 {
						csr[MSTATUS] |= MSTATUS_MIE_BIT // Restaura MIE de MPIE
					} else {
						csr[MSTATUS] &^= MSTATUS_MIE_BIT
					}
					csr[MSTATUS] |= MSTATUS_MPIE_BIT // Seta MPIE
					proximoPC = csr[MEPC]
				default:
					gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
					proximoPC = pc
				}
			case 0b001: // csrrw
				valorTemp := csr[csrAddr]
				csr[csrAddr] = uint32(x[rs1])
				if rd != 0 {
					x[rd] = int32(valorTemp)
				}
				fmt.Fprintf(writer, "0x%08x:csrrw  %s,0x%03x,%s\n", pc, xLabel[rd], csrAddr, xLabel[rs1])
			case 0b010: // csrrs
				valorTemp := csr[csrAddr]
				csr[csrAddr] = valorTemp | uint32(x[rs1])
				if rd != 0 {
					x[rd] = int32(valorTemp)
				}
				fmt.Fprintf(writer, "0x%08x:csrrs  %s,0x%03x,%s\n", pc, xLabel[rd], csrAddr, xLabel[rs1])
			case 0b011: // csrrc
				valorTemp := csr[csrAddr]
				csr[csrAddr] = valorTemp &^ uint32(x[rs1])
				if rd != 0 {
					x[rd] = int32(valorTemp)
				}
				fmt.Fprintf(writer, "0x%08x:csrrc  %s,0x%03x,%s\n", pc, xLabel[rd], csrAddr, xLabel[rs1])
			case 0b101: // csrrwi
				valorTemp := csr[csrAddr]
				csr[csrAddr] = immU
				if rd != 0 {
					x[rd] = int32(valorTemp)
				}
				fmt.Fprintf(writer, "0x%08x:csrrwi %s,0x%03x,%d\n", pc, xLabel[rd], csrAddr, immU)
			case 0b110: // csrrsi
				valorTemp := csr[csrAddr]
				csr[csrAddr] = valorTemp | immU
				if rd != 0 {
					x[rd] = int32(valorTemp)
				}
				fmt.Fprintf(writer, "0x%08x:csrrsi %s,0x%03x,%d\n", pc, xLabel[rd], csrAddr, immU)
			case 0b111: // csrrci
				valorTemp := csr[csrAddr]
				csr[csrAddr] = valorTemp &^ immU
				if rd != 0 {
					x[rd] = int32(valorTemp)
				}
				fmt.Fprintf(writer, "0x%08x:csrrci %s,0x%03x,%d\n", pc, xLabel[rd], csrAddr, immU)
			default:
				gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
				proximoPC = pc
			}

		default:
			gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
			proximoPC = pc
		}

	fimLoop:
		pc = proximoPC
	}
	
	// Exibir estatísticas finais das caches
	icacheHitRate := float64(icache.hits) / float64(icache.accesses)
	dcacheHitRate := float64(dcache.hits) / float64(dcache.accesses)
	fmt.Fprintf(writer, "#cache_mem:istats    hit=%.4f\n", icacheHitRate)
	fmt.Fprintf(writer, "#cache_mem:dstats    hit=%.4f\n", dcacheHitRate)
}