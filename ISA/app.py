#!/usr/bin/env python3
import sys


def load_memory(input_file, mem_array, base_offset):
    current_address = 0
    with open(input_file, "r") as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            if line.startswith("@"):
                current_address = int(line[1:], 16)

            else:
                byte_strings = line.split()
                for byte_str in byte_strings:
                    byte_value = int(byte_str, 16)
                    if not (0 <= byte_value <= 255):
                        raise ValueError("Byte value out of range.")
                        
                    mem_idx = current_address - base_offset
                    if 0 <= mem_idx < len(mem_array):
                        mem_array[mem_idx] = byte_value
                    else:
                        print(f"Warning: Address 0x{current_address:08x} (index {mem_idx}) is out of allocated memory bounds (0-{len(mem_array)-1}). Skipping byte {byte_str}.")
                    current_address += 1
    print(f"Successfully loaded memory from {input_file}")


def main(argv):
    print("--------------------------------------------------------------------------------")

    # Iterating over arguments
    for i, arg in enumerate(argv):
        print(f"argv[{i}] = {arg}")

    if len(argv) == 2:
        input_file = sys.argv[1]
    elif len(argv) > 2:
        input_file = sys.argv[1]
        output_file = open(sys.argv[2], "w")

    # Memory offset
    offset = 0x80000000
    
    # 32 registers initialized with zero
    x = [0] * 32
    x_label = [
        "zero", "ra", "sp", "gp", "tp", "t0", "t1", "t2", "s0", "s1",
        "a0", "a1", "a2", "a3", "a4", "a5", "a6", "a7", "s2", "s3",
        "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "t3", "t4",
        "t5", "t6"
    ]
    
    # Program counter initialized with memory offset
    pc = offset
    
    # 32 KiB memory for both data and instructions
    mem = bytearray(32 * 1024)
    
    # Load memory
    if len(argv) > 1:
        load_memory(input_file, mem, offset)
    else:
        # Manually filling memory for testing
        mem[0x80000000 - offset] = 0xef
        mem[0x80000001 - offset] = 0x00
        mem[0x80000002 - offset] = 0x00
        mem[0x80000003 - offset] = 0x10
        
        mem[0x80000100 - offset] = 0x13
        mem[0x80000101 - offset] = 0x10
        mem[0x80000102 - offset] = 0xf0
        mem[0x80000103 - offset] = 0x01

        mem[0x80000104 - offset] = 0x73
        mem[0x80000105 - offset] = 0x00
        mem[0x80000106 - offset] = 0x10
        mem[0x80000107 - offset] = 0x00

        mem[0x80000108 - offset] = 0x13
        mem[0x80000109 - offset] = 0x50
        mem[0x8000010a - offset] = 0x70
        mem[0x8000010b - offset] = 0x40

    print("--------------------------------------------------------------------------------")
    
    run = True 
    while run:
        # Read instruction from memory (4 byte alignment)
        instruction = (mem[pc - offset + 3] << 24) | (mem[pc - offset + 2] << 16) | (mem[pc - offset + 1] << 8) | mem[pc - offset]

         # Extract instruction fields
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

        # Checking instruction opcode
        if opcode == 0b0010011:  # I type (0010011)
            # slli (funct3 == 001 and funct7 == 0000000)
            if funct3 == 0b001 and funct7 == 0b0000000:
                data = (x[rs1] << uimm) & 0xFFFFFFFF
                print(f"0x{pc:08x}:slli   {x_label[rd]},{x_label[rs1]},{uimm}  {x_label[rd]}=0x{x[rs1]:08x}<<{uimm}=0x{data:08x}")
                if rd != 0:
                    x[rd] = data
        
        elif opcode == 0b1110011:  # I type (1110011)
            # ebreak (funct3 == 000 and imm == 1)
            if funct3 == 0b000 and imm == 1:
                # Outputting instruction to console
                print(f"0x{pc:08x}:ebreak")
                # Retrieving previous and next instructions
                prev_instr_pc = pc - 4
                next_instr_pc = pc + 4
                previous_instruction = 0
                next_instruction = 0
                # Read previous instruction if within bounds
                prev_mem_idx = prev_instr_pc - offset
                if 0 <= prev_mem_idx < len(mem) - 3:
                    prev_bytes = mem[prev_mem_idx : prev_mem_idx + 4]
                    previous_instruction = int.from_bytes(prev_bytes, byteorder='little')
                # Read next instruction if within bounds
                next_mem_idx = next_instr_pc - offset
                if 0 <= next_mem_idx < len(mem) - 3:
                    next_bytes = mem[next_mem_idx : next_mem_idx + 4]
                    next_instruction = int.from_bytes(next_bytes, byteorder='little')
                # Halting condition
                if previous_instruction == 0x01f01013 and next_instruction == 0x40705013:
                    run = False

        elif opcode == 0b1101111:  # J type (1101111)
            # Performing sign extension in immediate field
            simm = (0xFFF00000 | imm20) if (imm20 >> 19) & 1 else imm20
            # Calculating operation address
            address = pc + (simm << 1) & 0xFFFFFFFF
            # Outputting instruction to console
            print(f"0x{pc:08x}:jal    {x_label[rd]},0x{imm:05x}    pc=0x{address:08x},{x_label[rd]}=0x{(pc + 4) & 0xFFFFFFFF:08x}")
            # Updating register if not x[0] (zero)
            if rd != 0:
                x[rd] = (pc + 4) & 0xFFFFFFFF
             # Setting next pc minus 4
            pc = (address - 4) & 0xFFFFFFFF
        
        else:
            # Unknown opcode
            print(f"error: unknown instruction opcode 0b{opcode:07b} (0x{opcode:02x}) at pc = 0x{pc:08x}")
            run = False
        
        # Incrementing pc by 4 (and ensure it's 32-bit)
        pc = (pc + 4) & 0xFFFFFFFF

    if len(argv) > 2:
        output_file.close()
    print("--------------------------------------------------------------------------------")


if __name__ == "__main__":
    main(sys.argv)
