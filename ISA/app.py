# #!/usr/bin/env python3
import sys

def load_memory(input_file, mem, offset):
    address = 0
    with open(input_file, "r") as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            if line.startswith("@"):
                address = int(line[1:], 16)
            else:
                byte_strings = line.split()
                for byte_str in byte_strings:
                    byte_value = int(byte_str, 16)
                    mem_idx = address - offset
                    if 0 <= mem_idx < len(mem):
                        mem[mem_idx] = byte_value
                    address += 1
    print(f"Successfully loaded memory from {input_file}")


def read_instructions(mem, pc, offset):
    return (mem[pc - offset + 3] << 24) | (mem[pc - offset + 2] << 16) | (mem[pc - offset + 1] << 8) | mem[pc - offset]


def main(argv):
    print("--------------------------------------------------------------------------------")

    for i, arg in enumerate(argv):
        print(f"argv[{i}] = {arg}")

    end_input_file = sys.argv[1]
    # end_output_file = sys.argv[2]

    # Memoria offset
    offset = 0x80000000
    
    # 32 registradores inicializados com 0
    x = [0] * 32
    x_label = [
        "zero", "ra", "sp", "gp", "tp", "t0", "t1", "t2", "s0", "s1",
        "a0", "a1", "a2", "a3", "a4", "a5", "a6", "a7", "s2", "s3",
        "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "t3", "t4",
        "t5", "t6"
    ]
    
    # pc
    pc = offset
    
    # vetor de mem 32bytes
    mem = bytearray(32 * 1024)
    
    # carregar arquivo input na memoria
    load_memory(end_input_file, mem, offset)

    print("--------------------------------------------------------------------------------")
    
    run = True 
    while run:
        # Lendo instruções da memoria (4 byte)
        instruction = read_instructions(mem, pc, offset)

        # Campos de instruções
        opcode = instruction & 0b1111111
        funct7 = (instruction >> 25) & 0x7F
        imm = instruction >> 20
        uimm = (instruction >> 20) & 0x1F
        rs1 = (instruction >> 15) & 0x1F
        funct3 = (instruction >> 12) & 0x7
        rd = (instruction >> 7) & 0x1F
        imm20 = (
            ((instruction >> 31) & 0x1) << 19 |
            ((instruction >> 12) & 0xFF) << 11 |
            ((instruction >> 20) & 0x1) << 10 |
            ((instruction >> 21) & 0x3FF)
        )

        # Checando instruções pelo opcode
        if opcode == 0b0010011:  # I type (0010011)
            # slli (funct3 == 001 and funct7 == 0000000)
            if funct3 == 0b001 and funct7 == 0b0000000:
                data = (x[rs1] << uimm) & 0xFFFFFFFF
                
                # Imprimindo instruções
                print(f"0x{pc:08x}:slli   {x_label[rd]},{x_label[rs1]},{uimm}  {x_label[rd]}=0x{x[rs1]:08x}<<{uimm}=0x{data:08x}")
                
                if rd != 0:
                    x[rd] = data
        
        elif opcode == 0b1110011:  # I type (1110011)
            # ebreak (funct3 == 000 and imm == 1)
            if funct3 == 0b000 and imm == 1:
                # Imprimindo instruções
                print(f"0x{pc:08x}:ebreak")

                previous_instruction = int.from_bytes(mem[((pc - 4) - offset) : ((pc - 4) - offset) + 4], byteorder='little')
                next_instruction = int.from_bytes(mem[((pc + 4) - offset) : ((pc + 4) - offset) + 4], byteorder='little')
                
                ## Recuperando instruções anteriores e seguintes
                # prev_instr_pc = pc - 4
                # next_instr_pc = pc + 4
                # previous_instruction = 0
                # next_instruction = 0
                
                # # Leia as instruções anteriores
                # prev_mem_idx = prev_instr_pc - offset
                # if 0 <= prev_mem_idx < len(mem) - 3:
                #     prev_bytes = mem[prev_mem_idx : prev_mem_idx + 4]
                #     previous_instruction = int.from_bytes(prev_bytes, byteorder='little')
                
                # # Leia a próxima instrução
                # next_mem_idx = next_instr_pc - offset
                # if 0 <= next_mem_idx < len(mem) - 3:
                #     next_bytes = mem[next_mem_idx : next_mem_idx + 4]
                #     next_instruction = int.from_bytes(next_bytes, byteorder='little')
                
                # Condição de parada
                if previous_instruction == 0x01f01013 and next_instruction == 0x40705013:
                    run = False

        elif opcode == 0b1101111:  # J type (1101111)
            # Executando extensão de sinalização em campo imediato
            simm = (0xFFF00000 | imm20) if (imm20 >> 19) & 1 else imm20
            
            # Calculando endereço de operação
            address = pc + (simm << 1) & 0xFFFFFFFF
            
           # Imprimindo instruções
            print(f"0x{pc:08x}:jal    {x_label[rd]},0x{imm:05x}    pc=0x{address:08x},{x_label[rd]}=0x{(pc + 4) & 0xFFFFFFFF:08x}")
            
            # Updating register if not x[0] (zero)
            if rd != 0:
                x[rd] = (pc + 4) & 0xFFFFFFFF
                
             # Setando novo pc
            pc = (address - 4) & 0xFFFFFFFF
        
        else:
            # Opcode desconhecido
            print(f"error: unknown instruction opcode 0b{opcode:07b} (0x{opcode:02x}) at pc = 0x{pc:08x}")
            run = False
        
        # Incrementando pc (32-bit)
        pc = (pc + 4) & 0xFFFFFFFF

    print("--------------------------------------------------------------------------------")


if __name__ == "__main__":
    main(sys.argv)
