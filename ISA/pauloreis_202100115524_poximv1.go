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

func lerInstrucao(mem []byte, pc, offset uint32) uint32 {
	idxMem := pc - offset
	return binary.LittleEndian.Uint32(mem[idxMem : idxMem+4])
}

func estenderSinal(valor uint32, bits uint) int32 {
	shift := 32 - bits
	return int32(valor<<shift) >> shift
}

func main() {
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

	carregarMemoria(caminhoArquivoEntrada, mem, offset)

	executando := true
	for executando {
		x[0] = 0

		instrucao := lerInstrucao(mem, pc, offset)

		opcode := instrucao & 0x7F
		rd := (instrucao >> 7) & 0x1F
		rs1 := (instrucao >> 15) & 0x1F
		rs2 := (instrucao >> 20) & 0x1F
		funct3 := (instrucao >> 12) & 0x7
		funct7 := (instrucao >> 25) & 0x7F

		proximoPC := pc + 4

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
			resultado := pc + immU
			fmt.Fprintf(writer, "0x%08x:auipc  %s,0x%05x   rd=0x%08x+0x%08x=0x%08x\n", pc, xLabel[rd], immU>>12, pc, immU, resultado)
			if rd != 0 {
				x[rd] = int32(resultado)
			}

		case 0b0000011:
			immI := instrucao >> 20
			immSinalI := estenderSinal(immI, 12)
			enderecoMem := uint32(x[rs1]) + uint32(immSinalI)
			idxMem := enderecoMem - offset

			var data int32
			inst := ""
			switch funct3 {
			case 0b000: // lb
				inst = "lb"
				data = int32(int8(mem[idxMem]))
			case 0b100: // lbu
				inst = "lbu"
				data = int32(mem[idxMem])
			case 0b001: // lh
				inst = "lh"
				data = int32(int16(binary.LittleEndian.Uint16(mem[idxMem : idxMem+2])))
			case 0b101: // lhu
				inst = "lhu"
				data = int32(binary.LittleEndian.Uint16(mem[idxMem : idxMem+2]))
			case 0b010: // lw
				inst = "lw"
				data = int32(binary.LittleEndian.Uint32(mem[idxMem : idxMem+4]))
			}

			fmt.Fprintf(writer, "0x%08x:%-7s%s,0x%03x(%s)   %s=mem[0x%08x]=0x%08x\n", pc, inst, xLabel[rd], immSinalI&0xFFF, xLabel[rs1], xLabel[rd], enderecoMem, uint32(data))
			if rd != 0 {
				x[rd] = data
			}

		case 0b0100011:
			bitsImmS := ((instrucao>>25)&0x7F)<<5 | ((instrucao >> 7) & 0x1F)
			immSinalS := estenderSinal(bitsImmS, 12)
			enderecoMem := uint32(x[rs1]) + uint32(immSinalS)
			idxMem := enderecoMem - offset

			inst := ""
			stringOperacao := ""
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
			}

			fmt.Fprintf(writer, "0x%08x:%-7s%s,0x%03x(%s)   mem[0x%08x]=%s\n", pc, inst, xLabel[rs2], immSinalS&0xFFF, xLabel[rs1], enderecoMem, stringOperacao)

		case 0b0110011:
			var data int32
			inst := ""
			stringOperacao := ""
			shamt := uint32(x[rs2]) & 0x1F

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
					inst, stringOperacao = "sll", fmt.Sprintf("0x%08x<<%d", uint32(x[rs1]), shamt)
					data = x[rs1] << shamt
				case 0b101:
					if funct7 == 0 { // srl
						inst, stringOperacao = "srl", fmt.Sprintf("0x%08x>>%d", uint32(x[rs1]), shamt)
						data = int32(uint32(x[rs1]) >> shamt)
					} else { // sra
						inst, stringOperacao = "sra", fmt.Sprintf("0x%08x>>%d", uint32(x[rs1]), shamt)
						data = x[rs1] >> shamt
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
			fmt.Fprintf(writer, "0x%08x:%-7s%s,%s,%s   %s\n", pc, inst, xLabel[rd], xLabel[rs1], xLabel[rs2], stringOperacao)
			if rd != 0 {
				x[rd] = data
			}

		case 0b0010011:
			immI := instrucao >> 20
			immSinalI := estenderSinal(immI, 12)
			shamt := (instrucao >> 20) & 0x1F

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
				inst, stringOperacao = "slli", fmt.Sprintf("0x%08x<<%d", uint32(x[rs1]), shamt)
				data = x[rs1] << shamt
			case 0b101:
				if funct7 == 0 { // srli
					inst, stringOperacao = "srli", fmt.Sprintf("0x%08x>>%d", uint32(x[rs1]), shamt)
					data = int32(uint32(x[rs1]) >> shamt)
				} else { // srai
					inst, stringOperacao = "srai", fmt.Sprintf("0x%08x>>%d", uint32(x[rs1]), shamt)
					data = x[rs1] >> shamt
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
			}

			imediatoStr := fmt.Sprintf("0x%03x", immSinalI&0xFFF)
			if funct3 == 0b001 || funct3 == 0b101 {
				imediatoStr = fmt.Sprintf("%d", shamt)
			}
			fmt.Fprintf(writer, "0x%08x:%-7s%s,%s,%s   %s\n", pc, inst, xLabel[rd], xLabel[rs1], imediatoStr, stringOperacao)
			if rd != 0 {
				x[rd] = data
			}

		case 0b1100011:
			bitsImmB := ((instrucao >> 8) & 0xF) << 1   // imm[4:1]
			bitsImmB |= ((instrucao >> 25) & 0x3F) << 5 // imm[10:5]
			bitsImmB |= ((instrucao >> 7) & 1) << 11    // imm[11]
			bitsImmB |= ((instrucao >> 31) & 1) << 12   // imm[12]
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
				inst, charOperacao = "bltu", "<"
				if uint32(x[rs1]) < uint32(x[rs2]) {
					desviar = true
				}
			case 0b111: // bgeu
				inst, charOperacao = "bgeu", ">="
				if uint32(x[rs1]) >= uint32(x[rs2]) {
					desviar = true
				}
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
			bitsImmJ := ((instrucao >> 21) & 0x3FF) << 1 // imm[10:1]
			bitsImmJ |= ((instrucao >> 20) & 0x1) << 11  // imm[11]
			bitsImmJ |= ((instrucao >> 12) & 0xFF) << 12 // imm[19:12]
			bitsImmJ |= ((instrucao >> 31) & 1) << 20    // imm[20]
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

		case 0b1110011: // ebreak
			if funct3 == 0 && (instrucao>>20) == 1 {
				fmt.Fprintf(writer, "0x%08x:ebreak\n", pc)
				executando = false
			} else {
				fmt.Fprintf(writer, "error: unknown instruction opcode at pc = (0x%08x)\n", pc)
				executando = false
			}

		default:
			fmt.Fprintf(writer, "error: unknown instruction opcode at pc = (0x%08x)\n", pc)
			executando = false
		}

		pc = proximoPC
	}
}
