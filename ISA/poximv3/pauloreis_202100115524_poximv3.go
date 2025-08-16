package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	CACHE_CAPACITY = 256 // bytes
	BLOCK_SIZE     = 16  // bytes (4 words of 4 bytes)
	ASSOCIATIVITY  = 2   // 2-way
	NUM_BLOCKS     = CACHE_CAPACITY / BLOCK_SIZE
	NUM_SETS       = NUM_BLOCKS / ASSOCIATIVITY
	OFFSET_BITS    = 4 // log2(16)
	INDEX_BITS     = 3 // log2(8)
	TAG_BITS       = 32 - INDEX_BITS - OFFSET_BITS
	OFFSET_MASK    = (1 << OFFSET_BITS) - 1
	INDEX_MASK     = ((1 << INDEX_BITS) - 1) << OFFSET_BITS
)

type CacheLine struct {
	Valid bool
	Tag   uint32
	Age   int
	Data  []byte
}

type CacheSet []CacheLine

type Cache struct {
	Name      string
	Sets      []CacheSet
	Hits      int
	Misses    int
	IsICache  bool
	mainMem   []byte
	memOffset uint32
	writer    io.Writer
}

func NewCache(name string, isICache bool, mainMem []byte, memOffset uint32, writer io.Writer) *Cache {
	cache := &Cache{
		Name:      name,
		Sets:      make([]CacheSet, NUM_SETS),
		IsICache:  isICache,
		mainMem:   mainMem,
		memOffset: memOffset,
		writer:    writer,
	}
	for i := range cache.Sets {
		cache.Sets[i] = make(CacheSet, ASSOCIATIVITY)
		for j := range cache.Sets[i] {
			cache.Sets[i][j].Data = make([]byte, BLOCK_SIZE)
		}
	}
	return cache
}

func (s CacheSet) updateLRU(hitIndex int) {
	for i := range s {
		if i == hitIndex {
			s[i].Age = 0
		} else {
			s[i].Age = 1
		}
	}
}

func (s CacheSet) findLRU_Way() int {
	for i, line := range s {
		if !line.Valid {
			return i
		}
		if line.Age == 1 {
			return i
		}
	}
	return 0
}

func (c *Cache) logState(prefix string, address uint32, index uint32) {
	set := c.Sets[index]
	valid := fmt.Sprintf("{%d,%d}", boolToInt(set[0].Valid), boolToInt(set[1].Valid))
	age := fmt.Sprintf("{%d,%d}", set[0].Age, set[1].Age)
	id := fmt.Sprintf("{0x%06x,0x%06x}", set[0].Tag, set[1].Tag)
	fmt.Fprintf(c.writer, "#cache_mem:%s 0x%08x line=%d,valid=%s,age=%s,id=%s\n", prefix, address, index, valid, age, id)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Access performs a read or write operation on the cache.
// It now returns an additional boolean `hasFault` to indicate a memory bounds error.
func (c *Cache) Access(address uint32, size int, dataToWrite []byte, isWrite bool) (data []byte, isHit bool, hasFault bool) {
	tag := address >> (INDEX_BITS + OFFSET_BITS)
	index := (address & INDEX_MASK) >> OFFSET_BITS
	offset := address & OFFSET_MASK
	blockStartAddr := address & ^uint32(OFFSET_MASK)

	set := c.Sets[index]

	// 1. Check for a cache hit
	for i, line := range set {
		if line.Valid && line.Tag == tag {
			c.Hits++
			set.updateLRU(i)

			if isWrite { // Data Write Hit
				memIdxWrite := address - c.memOffset
				if memIdxWrite+uint32(size) > uint32(len(c.mainMem)) {
					return nil, true, true // Memory Access Fault
				}
				logPrefix := "dwh"
				copy(line.Data[offset:offset+uint32(size)], dataToWrite)
				copy(c.mainMem[memIdxWrite:memIdxWrite+uint32(size)], dataToWrite)
				c.logState(logPrefix, address, index)
				return nil, true, false
			} else { // Instruction or Data Read Hit
				logPrefix := "irh"
				if !c.IsICache {
					logPrefix = "drh"
				}
				c.logState(logPrefix, address, index)
				return line.Data[offset : offset+uint32(size)], true, false
			}
		}
	}

	// 2. Cache Miss
	c.Misses++

	if isWrite { // Data Write Miss
		memIdxWrite := address - c.memOffset
		if memIdxWrite+uint32(size) > uint32(len(c.mainMem)) {
			return nil, false, true // Memory Access Fault
		}
		logPrefix := "dwm"
		copy(c.mainMem[memIdxWrite:memIdxWrite+uint32(size)], dataToWrite)
		c.logState(logPrefix, address, index)
		return nil, false, false
	} else { // Instruction or Data Read Miss
		logPrefix := "irm"
		if !c.IsICache {
			logPrefix = "drm"
		}
		c.logState(logPrefix, address, index)

		memIdxRead := blockStartAddr - c.memOffset
		if memIdxRead+BLOCK_SIZE > uint32(len(c.mainMem)) {
			return nil, false, true // Memory Access Fault
		}

		wayToReplace := set.findLRU_Way()
		line := &set[wayToReplace]

		copy(line.Data, c.mainMem[memIdxRead:memIdxRead+BLOCK_SIZE])

		line.Valid = true
		line.Tag = tag
		set.updateLRU(wayToReplace)

		return line.Data[offset : offset+uint32(size)], false, false
	}
}

func (c *Cache) printStats() {
	totalAccesses := c.Hits + c.Misses
	if totalAccesses == 0 {
		return
	}
	hitRate := float64(c.Hits) / float64(totalAccesses)

	statsPrefix := "istats"
	if !c.IsICache {
		statsPrefix = "dstats"
	}
	fmt.Fprintf(c.writer, "#cache_mem:%s hit=%.4f\n", statsPrefix, hitRate)
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

func estenderSinal(valor uint32, bits uint) int32 {
	desloca := 32 - bits
	return int32(valor<<desloca) >> desloca
}

// Constants for CSRs, Exceptions, and Interrupts
const (
	MSTATUS                      = 0x300
	MIE                          = 0x304
	MTVEC                        = 0x305
	MEPC                         = 0x341
	MCAUSE                       = 0x342
	MTVAL                        = 0x343
	MIP                          = 0x344
	MSTATUS_MIE_BIT              = 1 << 3
	MSTATUS_MPIE_BIT             = 1 << 7
	MIP_MTIP_BIT                 = 1 << 7
	MIP_MSIP_BIT                 = 1 << 3
	MIP_MEIP_BIT                 = 1 << 11
	EXC_INSTRUCTION_ACCESS_FAULT = 1
	EXC_ILLEGAL_INSTRUCTION      = 2
	EXC_LOAD_ACCESS_FAULT        = 5
	EXC_STORE_ACCESS_FAULT       = 7
	EXC_ECALL_FROM_M_MODE        = 11
	INT_MACHINE_SOFTWARE         = 3
	INT_MACHINE_TIMER            = 7
	INT_MACHINE_EXTERNAL         = 11
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

	exceptionNames := map[uint32]string{
		EXC_INSTRUCTION_ACCESS_FAULT: "instruction_fault",
		EXC_ILLEGAL_INSTRUCTION:      "illegal_instruction",
		EXC_LOAD_ACCESS_FAULT:        "load_fault",
		EXC_STORE_ACCESS_FAULT:       "store_fault",
		EXC_ECALL_FROM_M_MODE:        "environment_call",
	}
	interruptNames := map[uint32]string{
		INT_MACHINE_SOFTWARE: "software",
		INT_MACHINE_TIMER:    "timer",
		INT_MACHINE_EXTERNAL: "external",
	}

	csr := make(map[uint32]uint32)
	csr[MSTATUS], csr[MTVEC], csr[MIE], csr[MIP] = 0, 0, 0, 0

	carregarMemoria(caminhoArquivoEntrada, mem, offset)

	// Initialize caches
	iCache := NewCache("I", true, mem, offset, writer)
	dCache := NewCache("D", false, mem, offset, writer)

	gerarExcecao := func(codigoTrap, valorTrap uint32, isInterrupt bool) {
		csr[MEPC] = pc
		csr[MTVAL] = valorTrap
		if isInterrupt {
			csr[MCAUSE] = (1 << 31) | codigoTrap
		} else {
			csr[MCAUSE] = codigoTrap
		}
		if (csr[MSTATUS] & MSTATUS_MIE_BIT) != 0 {
			csr[MSTATUS] |= MSTATUS_MPIE_BIT
		} else {
			csr[MSTATUS] &^= MSTATUS_MPIE_BIT
		}
		csr[MSTATUS] &^= MSTATUS_MIE_BIT
		var eventName, eventType string
		var ok bool
		if isInterrupt {
			eventType = "interrupt"
			eventName, ok = interruptNames[codigoTrap]
		} else {
			eventType = "exception"
			eventName, ok = exceptionNames[codigoTrap]
		}
		if !ok {
			eventName = "Unknown"
		}
		fmt.Fprintf(writer, ">%s:%s 			cause=0x%08x,epc=0x%08x,tval=0x%08x\n", eventType, eventName, csr[MCAUSE], csr[MEPC], csr[MTVAL])
		pc = csr[MTVEC] & ^uint32(0x3)
	}

	executando := true
	for executando {
		x[0] = 0

		// Interrupt handling
		mieGlobal := (csr[MSTATUS] & MSTATUS_MIE_BIT) != 0
		interrupcoesPendentes := csr[MIE] & csr[MIP]
		if mieGlobal && interrupcoesPendentes != 0 {
			var interruptCode uint32
			if (interrupcoesPendentes & MIP_MEIP_BIT) != 0 {
				interruptCode = INT_MACHINE_EXTERNAL
			} else if (interrupcoesPendentes & MIP_MSIP_BIT) != 0 {
				interruptCode = INT_MACHINE_SOFTWARE
			} else if (interrupcoesPendentes & MIP_MTIP_BIT) != 0 {
				interruptCode = INT_MACHINE_TIMER
			}
			if interruptCode != 0 {
				gerarExcecao(interruptCode, 0, true)
				if interruptCode == INT_MACHINE_TIMER {
					csr[MIP] &^= MIP_MTIP_BIT
				}
				continue
			}
		}

		// Instruction Fetch via I-Cache
		instructionBytes, _, hasFault := iCache.Access(pc, 4, nil, false)
		if hasFault {
			gerarExcecao(EXC_INSTRUCTION_ACCESS_FAULT, pc, false)
			continue
		}
		instrucao := binary.LittleEndian.Uint32(instructionBytes)

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
			var data int32
			inst := ""
			var readSize int
			switch funct3 {
			case 0b000:
				inst, readSize = "lb", 1
			case 0b100:
				inst, readSize = "lbu", 1
			case 0b001:
				inst, readSize = "lh", 2
			case 0b101:
				inst, readSize = "lhu", 2
			case 0b010:
				inst, readSize = "lw", 4
			default:
				gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
				proximoPC = pc
				goto fimLoop
			}

			// Data Read via D-Cache
			bytesLidos, _, hasFault_load := dCache.Access(enderecoMem, readSize, nil, false)
			if hasFault_load {
				gerarExcecao(EXC_LOAD_ACCESS_FAULT, enderecoMem, false)
				proximoPC = pc
				goto fimLoop
			}

			switch funct3 {
			case 0b000:
				data = int32(int8(bytesLidos[0]))
			case 0b100:
				data = int32(bytesLidos[0])
			case 0b001:
				data = int32(int16(binary.LittleEndian.Uint16(bytesLidos)))
			case 0b101:
				data = int32(binary.LittleEndian.Uint16(bytesLidos))
			case 0b010:
				data = int32(binary.LittleEndian.Uint32(bytesLidos))
			}
			fmt.Fprintf(writer, "0x%08x:%-7s%s,0x%03x(%s)   %s=mem[0x%08x]=0x%08x\n", pc, inst, xLabel[rd], immSinalI&0xFFF, xLabel[rs1], xLabel[rd], enderecoMem, uint32(data))
			if rd != 0 {
				x[rd] = data
			}

		case 0b0100011: // Store instructions
			bitsImmS := ((instrucao>>25)&0x7F)<<5 | ((instrucao >> 7) & 0x1F)
			immSinalS := estenderSinal(bitsImmS, 12)
			enderecoMem := uint32(x[rs1]) + uint32(immSinalS)
			var dataToWrite []byte
			var writeSize int
			inst := ""
			stringOperacao := ""
			switch funct3 {
			case 0b000: // sb
				inst, writeSize = "sb", 1
				val := byte(x[rs2])
				stringOperacao = fmt.Sprintf("0x%02x", val)
				dataToWrite = []byte{val}
			case 0b001: // sh
				inst, writeSize = "sh", 2
				val := uint16(x[rs2])
				stringOperacao = fmt.Sprintf("0x%04x", val)
				dataToWrite = make([]byte, 2)
				binary.LittleEndian.PutUint16(dataToWrite, val)
			case 0b010: // sw
				inst, writeSize = "sw", 4
				val := uint32(x[rs2])
				stringOperacao = fmt.Sprintf("0x%08x", val)
				dataToWrite = make([]byte, 4)
				binary.LittleEndian.PutUint32(dataToWrite, val)
			default:
				gerarExcecao(EXC_ILLEGAL_INSTRUCTION, instrucao, false)
				proximoPC = pc
				goto fimLoop
			}

			// Data Write via D-Cache
			_, _, hasFault_store := dCache.Access(enderecoMem, writeSize, dataToWrite, true)
			if hasFault_store {
				gerarExcecao(EXC_STORE_ACCESS_FAULT, enderecoMem, false)
				proximoPC = pc
				goto fimLoop
			}
			fmt.Fprintf(writer, "0x%08x:%-7s%s,0x%03x(%s)   mem[0x%08x]=%s\n", pc, inst, xLabel[rs2], immSinalS&0xFFF, xLabel[rs1], enderecoMem, stringOperacao)

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
					}
				case 0b011: // sltu
					inst, stringOperacao = "sltu", fmt.Sprintf("(0x%08x<0x%08x) (unsigned)", uint32(x[rs1]), uint32(x[rs2]))
					if uint32(x[rs1]) < uint32(x[rs2]) {
						data = 1
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
				}
			case 0b011: // sltiu
				inst, stringOperacao = "sltiu", fmt.Sprintf("(0x%08x<%d)", uint32(x[rs1]), immSinalI)
				if uint32(x[rs1]) < uint32(immSinalI) {
					data = 1
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
					if (csr[MSTATUS] & MSTATUS_MPIE_BIT) != 0 {
						csr[MSTATUS] |= MSTATUS_MIE_BIT
					} else {
						csr[MSTATUS] &^= MSTATUS_MIE_BIT
					}
					csr[MSTATUS] |= MSTATUS_MPIE_BIT
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

	// Print final cache statistics
	dCache.printStats()
	iCache.printStats()
}
