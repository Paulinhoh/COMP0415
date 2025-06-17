#!/usr/bin/env python3
import sys

def carregar_memoria(caminho_arquivo, mem, offset):
    endereco = 0
    with open(caminho_arquivo, "r") as f:
        for linha in f:
            linha = linha.strip()
            if not linha:
                continue
            if linha.startswith("@"):
                endereco = int(linha[1:], 16)
            else:
                strings_de_bytes = linha.split()
                for string_do_byte in strings_de_bytes:
                    valor_do_byte = int(string_do_byte, 16)
                    idx_mem = endereco - offset
                    if 0 <= idx_mem < len(mem):
                        mem[idx_mem] = valor_do_byte
                    endereco += 1
    print(f"memoria carregada com sucesso de {caminho_arquivo}")


def ler_instrucao(mem, pc, offset):
    idx_mem = pc - offset
    return int.from_bytes(mem[idx_mem : idx_mem + 4], byteorder='little')

def estender_sinal(valor, bits):
    bit_de_sinal = 1 << (bits - 1)
    return (valor & (bit_de_sinal - 1)) - (valor & bit_de_sinal)

def main(argv):
    print("--------------------------------------------------------------------------------")

    caminho_arquivo_entrada = sys.argv[1]
    caminho_arquivo_saida = sys.argv[2]
    offset = 0x80000000
    
    x = [0] * 32
    x_label = [
        "zero", "ra", "sp", "gp", "tp", "t0", "t1", "t2", "s0", "s1",
        "a0", "a1", "a2", "a3", "a4", "a5", "a6", "a7", "s2", "s3",
        "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "t3", "t4",
        "t5", "t6"
    ]
    
    pc = offset
    mem = bytearray(32 * 1024)
    
    carregar_memoria(caminho_arquivo_entrada, mem, offset)

    print("--------------------------------------------------------------------------------")
    
    arquivo_saida = open(caminho_arquivo_saida, "w")
    executando = True 
    while executando:
        x[0] = 0
        
        instrucao = ler_instrucao(mem, pc, offset)

        opcode = instrucao & 0x7F
        rd = (instrucao >> 7) & 0x1F
        rs1 = (instrucao >> 15) & 0x1F
        rs2 = (instrucao >> 20) & 0x1F
        funct3 = (instrucao >> 12) & 0x7
        funct7 = (instrucao >> 25) & 0x7F
        
        proximo_pc = pc + 4

        # lui
        if opcode == 0b0110111:
            imm_u = instrucao & 0xFFFFF000
            resultado = imm_u
            print(f"0x{pc:08x}:lui    {x_label[rd]},{hex(imm_u >> 12)}          {x_label[rd]}=0x{resultado:08x}")
            arquivo_saida.write(f"0x{pc:08x}:lui    {x_label[rd]},{hex(imm_u >> 12)}          {x_label[rd]}=0x{resultado:08x}\n")
            if rd != 0: x[rd] = resultado

        # auipc
        elif opcode == 0b0010111:
            imm_u = instrucao & 0xFFFFF000
            resultado = (pc + imm_u) & 0xFFFFFFFF
            print(f"0x{pc:08x}:auipc  {x_label[rd]},0x{imm_u >> 12:x}          {x_label[rd]}=0x{pc:08x}+0x{imm_u:08x}=0x{resultado:08x}")
            arquivo_saida.write(f"0x{pc:08x}:auipc  {x_label[rd]},0x{imm_u >> 12:x}          {x_label[rd]}=0x{pc:08x}+0x{imm_u:08x}=0x{resultado:08x}\n")
            if rd != 0: x[rd] = resultado
            
        # jal
        elif opcode == 0b1101111:
            bits_imm_j = ((instrucao >> 31) & 1) << 20 | ((instrucao >> 12) & 0xFF) << 12 | ((instrucao >> 20) & 1) << 11 | ((instrucao >> 21) & 0x3FF) << 1
            imm_sinal_j = estender_sinal(bits_imm_j, 21)
            
            valor_rd = proximo_pc
            pc_alvo = (pc + imm_sinal_j) & 0xFFFFFFFF
            print(f"0x{pc:08x}:jal    {x_label[rd]},0x{imm_sinal_j & 0x1FFFFF:x}        pc=0x{pc_alvo:08x},{x_label[rd]}=0x{valor_rd:08x}")
            arquivo_saida.write(f"0x{pc:08x}:jal    {x_label[rd]},0x{imm_sinal_j & 0x1FFFFF:x}        pc=0x{pc_alvo:08x},{x_label[rd]}=0x{valor_rd:08x}\n")
            if rd != 0: x[rd] = valor_rd
            proximo_pc = pc_alvo

        # jalr
        elif opcode == 0b1100111:
            imm_i = instrucao >> 20
            imm_sinal_i = estender_sinal(imm_i, 12)
            
            valor_rd = proximo_pc
            endereco_alvo = (x[rs1] + imm_sinal_i) & ~1
            print(f"0x{pc:08x}:jalr   {x_label[rd]},{x_label[rs1]},0x{imm_i:x}       pc=0x{x[rs1]:08x}+{imm_sinal_i & 0xFFFFFFFF:08x},{x_label[rd]}=0x{valor_rd:08x}")
            arquivo_saida.write(f"0x{pc:08x}:jalr   {x_label[rd]},{x_label[rs1]},0x{imm_i:x}       pc=0x{x[rs1]:08x}+{imm_sinal_i & 0xFFFFFFFF:08x},{x_label[rd]}=0x{valor_rd:08x}\n")
            if rd != 0: x[rd] = valor_rd
            proximo_pc = endereco_alvo

        elif opcode == 0b1100011:
            bits_imm_b = ((instrucao >> 31) & 1) << 12 | ((instrucao >> 7) & 1) << 11 | ((instrucao >> 25) & 0x3F) << 5 | ((instrucao >> 8) & 0xF) << 1
            imm_sinal_b = estender_sinal(bits_imm_b, 13)
            
            desviar = False
            char_operacao = ""
            resultado_comparacao = 0
            inst = ""
            
            if funct3 == 0b000:   # beq
                inst = "beq"
                if x[rs1] == x[rs2]: desviar = True
                char_operacao = "=="
            elif funct3 == 0b001: # bne
                inst = "bne"
                if x[rs1] != x[rs2]: desviar = True
                char_operacao = "!="
            elif funct3 == 0b100: # blt
                inst = "blt"
                if x[rs1] < x[rs2]: desviar = True
                char_operacao = "<"
            elif funct3 == 0b101: # bge
                inst = "bge"
                if x[rs1] >= x[rs2]: desviar = True
                char_operacao = ">="
            elif funct3 == 0b110: # bltu
                inst = "bltu"
                if (x[rs1] & 0xFFFFFFFF) < (x[rs2] & 0xFFFFFFFF): desviar = True
                char_operacao = "<"
            elif funct3 == 0b111: # bgeu
                inst = "bgeu"
                if (x[rs1] & 0xFFFFFFFF) >= (x[rs2] & 0xFFFFFFFF): desviar = True
                char_operacao = ">="
            
            resultado_comparacao = 1 if desviar else 0
            pc_alvo = (pc + imm_sinal_b) if desviar else proximo_pc
            print(f"0x{pc:08x}:{inst:<7}{x_label[rs1]},{x_label[rs2]},0x{imm_sinal_b & 0x1FFF:x}         ({x[rs1]:08x}{char_operacao}{x[rs2]:08x})={resultado_comparacao}->pc=0x{pc_alvo:08x}")
            arquivo_saida.write(f"0x{pc:08x}:{inst:<7}{x_label[rs1]},{x_label[rs2]},0x{imm_sinal_b & 0x1FFF:x}         ({x[rs1]:08x}{char_operacao}{x[rs2]:08x})={resultado_comparacao}->pc=0x{pc_alvo:08x}\n")

            if desviar: proximo_pc = pc + imm_sinal_b
        
        elif opcode == 0b0000011:
            imm_i = instrucao >> 20
            imm_sinal_i = estender_sinal(imm_i, 12)
            endereco_mem = x[rs1] + imm_sinal_i
            idx_mem = endereco_mem - offset
            
            data = 0
            inst = ""
            if funct3 == 0b000:   # lb
                inst = "lb"
                data = estender_sinal(mem[idx_mem], 8)
            elif funct3 == 0b001: # lh
                inst = "lh"
                data = estender_sinal(int.from_bytes(mem[idx_mem:idx_mem+2], 'little'), 16)
            elif funct3 == 0b010: # lw
                inst = "lw"
                data = estender_sinal(int.from_bytes(mem[idx_mem:idx_mem+4], 'little'), 32)
            elif funct3 == 0b100: # lbu
                inst = "lbu"
                data = mem[idx_mem]
            elif funct3 == 0b101: # lhu
                inst = "lhu"
                data = int.from_bytes(mem[idx_mem:idx_mem+2], 'little')
            
            print(f"0x{pc:08x}:{inst:<7}{x_label[rd]},{imm_sinal_i}({x_label[rs1]})        {x_label[rd]}=mem[0x{endereco_mem:08x}]=0x{data:08x}")
            arquivo_saida.write(f"0x{pc:08x}:{inst:<7}{x_label[rd]},{imm_sinal_i}({x_label[rs1]})        {x_label[rd]}=mem[0x{endereco_mem:08x}]=0x{data:08x}\n")
            if rd != 0: x[rd] = data

        elif opcode == 0b0100011:
            bits_imm_s = ((instrucao >> 25) & 0x7F) << 5 | ((instrucao >> 7) & 0x1F)
            imm_sinal_s = estender_sinal(bits_imm_s, 12)
            endereco_mem = x[rs1] + imm_sinal_s
            idx_mem = endereco_mem - offset
            
            inst = ""
            if funct3 == 0b000:   # sb
                inst = "sb"
                mem[idx_mem] = x[rs2] & 0xFF
            elif funct3 == 0b001: # sh
                inst = "sh"
                mem[idx_mem:idx_mem+2] = (x[rs2] & 0xFFFF).to_bytes(2, 'little')
            elif funct3 == 0b010: # sw
                inst = "sw"
                mem[idx_mem:idx_mem+4] = (x[rs2] & 0xFFFFFFFF).to_bytes(4, 'little')
            
            print(f"0x{pc:08x}:{inst:<7}{x_label[rs2]},{imm_sinal_s}({x_label[rs1]})      mem[0x{endereco_mem:08x}]=0x{x[rs2]:08x}")
            arquivo_saida.write(f"0x{pc:08x}:{inst:<7}{x_label[rs2]},{imm_sinal_s}({x_label[rs1]})      mem[0x{endereco_mem:08x}]=0x{x[rs2]:08x}\n")
        
        elif opcode == 0b0010011:
            imm_i = instrucao >> 20
            imm_sinal_i = estender_sinal(imm_i, 12)
            shamt = rs2
            
            data = 0
            inst = ""
            string_operacao = ""

            if funct3 == 0b000:   # addi
                inst, string_operacao = "addi", f"0x{x[rs1]:08x}+0x{imm_sinal_i & 0xFFFFFFFF:08x}"
                data = x[rs1] + imm_sinal_i
            elif funct3 == 0b010: # slti
                inst, string_operacao = "slti", f"({x[rs1]:08x}<{imm_sinal_i & 0xFFFFFFFF:08x})"
                data = 1 if x[rs1] < imm_sinal_i else 0
            elif funct3 == 0b011: # sltiu
                inst, string_operacao = "sltiu", f"({x[rs1]:08x}<{imm_sinal_i & 0xFFFFFFFF:08x})"
                data = 1 if (x[rs1] & 0xFFFFFFFF) < (imm_sinal_i & 0xFFFFFFFF) else 0
            elif funct3 == 0b100: # xori
                inst, string_operacao = "xori", f"0x{x[rs1]:08x}^0x{imm_sinal_i & 0xFFFFFFFF:08x}"
                data = x[rs1] ^ imm_sinal_i
            elif funct3 == 0b110: # ori
                inst, string_operacao = "ori", f"0x{x[rs1]:08x}|0x{imm_sinal_i & 0xFFFFFFFF:08x}"
                data = x[rs1] | imm_sinal_i
            elif funct3 == 0b111: # andi
                inst, string_operacao = "andi", f"0x{x[rs1]:08x}&0x{imm_sinal_i & 0xFFFFFFFF:08x}"
                data = x[rs1] & imm_sinal_i
            elif funct3 == 0b001: # slli
                inst, string_operacao = "slli", f"0x{x[rs1]:08x}<<{shamt}"
                data = x[rs1] << shamt
            elif funct3 == 0b101:
                if funct7 == 0: # srli
                    inst, string_operacao = "srli", f"0x{x[rs1]:08x}>>{shamt}"
                    data = (x[rs1] & 0xFFFFFFFF) >> shamt
                else: # srai
                    inst, string_operacao = "srai", f"0x{x[rs1]:08x}>>{shamt}"
                    data = x[rs1] >> shamt

            data &= 0xFFFFFFFF
            print(f"0x{pc:08x}:{inst:<7}{x_label[rd]},{x_label[rs1]},{imm_sinal_i if funct3 not in [1, 5] else shamt}         {x_label[rd]}={string_operacao}=0x{data:08x}")
            arquivo_saida.write(f"0x{pc:08x}:{inst:<7}{x_label[rd]},{x_label[rs1]},{imm_sinal_i if funct3 not in [1, 5] else shamt}         {x_label[rd]}={string_operacao}=0x{data:08x}\n")
            if rd != 0: x[rd] = data
                
        elif opcode == 0b0110011:
            data = 0
            shamt = x[rs2] & 0x1F
            inst = ""
            string_operacao = ""

            if funct7 == 0b0000001:
                if funct3 == 0b000: # mul
                    inst, string_operacao = "mul", f"0x{x[rs1]:08x}*0x{x[rs2]:08x}"
                    data = x[rs1] * x[rs2]
            elif funct3 == 0b000:
                if funct7 == 0: # add
                    inst, string_operacao = "add", f"0x{x[rs1]:08x}+0x{x[rs2]:08x}"
                    data = x[rs1] + x[rs2]
                else: # SUB
                    inst, string_operacao = "sub", f"0x{x[rs1]:08x}-0x{x[rs2]:08x}"
                    data = x[rs1] - x[rs2]
            elif funct3 == 0b001: # sll
                inst, string_operacao = "sll", f"0x{x[rs1]:08x}<<{shamt}"
                data = x[rs1] << shamt
            elif funct3 == 0b010: # slt
                inst, string_operacao = "slt", f"({x[rs1]:08x}<{x[rs2]:08x})"
                data = 1 if x[rs1] < x[rs2] else 0
            elif funct3 == 0b100: # xor
                inst, string_operacao = "xor", f"0x{x[rs1]:08x}^{x[rs2]:08x}"
                data = x[rs1] ^ x[rs2]
            elif funct3 == 0b101:
                if funct7 == 0: # srl
                    inst, string_operacao = "srl", f"0x{x[rs1]:08x}>>{shamt}"
                    data = (x[rs1] & 0xFFFFFFFF) >> shamt
                else: # sra
                    inst, string_operacao = "sra", f"0x{x[rs1]:08x}>>{shamt}"
                    data = x[rs1] >> shamt
            elif funct3 == 0b110: # or
                inst, string_operacao = "or", f"0x{x[rs1]:08x}|0x{x[rs2]:08x}"
                data = x[rs1] | x[rs2]
            elif funct3 == 0b111: # and
                inst, string_operacao = "and", f"0x{x[rs1]:08x}&0x{x[rs2]:08x}"
                data = x[rs1] & x[rs2]

            data &= 0xFFFFFFFF
            print(f"0x{pc:08x}:{inst:<7}{x_label[rd]},{x_label[rs1]},{x_label[rs2]}            {x_label[rd]}={string_operacao}=0x{data:08x}")
            arquivo_saida.write(f"0x{pc:08x}:{inst:<7}{x_label[rd]},{x_label[rs1]},{x_label[rs2]}            {x_label[rd]}={string_operacao}=0x{data:08x}\n")
            if rd != 0: x[rd] = data
        
        # ebreak
        elif opcode == 0b1110011 and funct3 == 0 and (instrucao >> 20) == 1:
            print(f"0x{pc:08x}:ebreak")
            arquivo_saida.write(f"0x{pc:08x}:ebreak\n")
            executando = False
        
        else:
            print(f"error: unknown instruction opcode at pc = 0x{pc:08x})")
            arquivo_saida.write(f"error: unknown instruction opcode at pc = 0x{pc:08x})\n")
            executando = False
        
        pc = proximo_pc
    
    print("--------------------------------------------------------------------------------")
    arquivo_saida.close()
    
if __name__ == "__main__":
    main(sys.argv)
