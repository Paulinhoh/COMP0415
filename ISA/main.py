import sys

def main(argv):
    print("--------------------------------------------------------------------------------")
    for i, arg in enumerate(argv):
        print(f"argv[{i}] = {arg}")
    
    # input = open(sys.args[1], "r")
    # output = open(sys.args[2], "w")

    # Memory offset
    offset = 0x80000000
        
    # 32 registers initialized with zero
    x = [0] * 32
    x_label = [
        "zero", "ra", "sp", "gp", "tp", "t0", "t1", "t2",
        "s0", "s1", "a0", "a1", "a2", "a3", "a4", "a5",
        "a6", "a7", "s2", "s3", "s4", "s5", "s6", "s7",
        "s8", "s9", "s10", "s11", "t3", "t4", "t5", "t6"
    ]
    
    # Program counter initialized with memory offset
    pc = offset
        
    # 32 KiB memory for both data and instructions
    mem = bytearray(32 * 1024)
    
    # Manually filling memory for testing purposes
    # 80000000: 100000ef jal    0x80000100
    mem[0x80000000 - offset] = 0xef
    mem[0x80000001 - offset] = 0x00
    mem[0x80000002 - offset] = 0x00
    mem[0x80000003 - offset] = 0x10
        
    # 80000100: 01f01013 slli   zero,zero,0x1f
    mem[0x80000100 - offset] = 0x13
    mem[0x80000101 - offset] = 0x10
    mem[0x80000102 - offset] = 0xf0
    mem[0x80000103 - offset] = 0x01
        
    # 80000104: 00100073 ebreak
    mem[0x80000104 - offset] = 0x73
    mem[0x80000105 - offset] = 0x00
    mem[0x80000106 - offset] = 0x10
    mem[0x80000107 - offset] = 0x00
        
    # 80000108: 40705013 srai   zero,zero,0x7
    mem[0x80000108 - offset] = 0x13
    mem[0x80000109 - offset] = 0x50
    mem[0x8000010a - offset] = 0x70
    mem[0x8000010b - offset] = 0x40
        
    # Run condition
    run = True
    
    while run:
        # Read instruction from memory (4 byte alignment)
        instruction = (mem[pc - offset + 3] << 24) | (mem[pc - offset + 2] << 16) | (mem[pc - offset + 1] << 8) | mem[pc - offset]
        
        # Extract instruction fields
        opcode = instruction & 0b1111111
        funct7 = instruction >> 25
        imm = instruction >> 20
        uimm = (instruction & (0b11111 << 20)) >> 20
        rs1 = (instruction & (0b11111 << 15)) >> 15
        funct3 = (instruction & (0b111 << 12)) >> 12
        rd = (instruction & (0b11111 << 7)) >> 7
        imm20 = ((instruction >> 31) << 19) | (((instruction & (0b11111111 << 12)) >> 12) << 11) | (((instruction & (0b1 << 20)) >> 20) << 10) | ((instruction & (0b1111111111 << 21)) >> 21)

        # Decode and execute instruction
        match opcode:
            case 0b0010011: # I type (0010011)
                # slli (funct3 == 001 and funct7 == 0000000)
                if funct3 == 0b001 and funct7 == 0b0000000:
                    # Calculating operation data
                    data = x[rs1] << imm
                    # Outputting instruction to console
                    print(f"0x{pc:08x}:slli   {x_label[rd]},{x_label[rs1]},{imm}  {x_label[rd]}=0x{x[rs1]:08x}<<{imm}=0x{data:08x}")
                # Breaking case
                break   
            case 0b1110011: # I type (1110011)
                # ebreak (funct3 == 000 and imm == 1)
                if funct3 == 0b000 and imm == 1:
                    # Outputting instruction to console
                    print(f"0x{pc:08x}:ebreak")
                    # Retrieving previous and next instructions
                    previous_i = int.from_bytes(mem[pc-4-offset:pc-offset], byteorder='little')
                    next_i = int.from_bytes(mem[pc+4-offset:pc+8-offset], byteorder='little')
                    # Halting condition
                    if previous_i == 0x01f01013 and next_i == 0x40705013:
                        run = False    
                # Breaking case
                break
            case 0b1101111: # J type (1101111)
                # Performing sign extension in immediate field
                simm = (0xFFF00000 | imm20) if (imm20 >> 19) else imm20
                # Calculating operation address
                address = pc + (simm << 1)
                # Outputting instruction to console
                print(f"0x{pc:08x}:jal    {x_label[rd]},0x{imm:05x}    pc=0x{address:08x},{x_label[rd]}=0x{pc + 4:08x}")
                # Updating register if not x[0] (zero)
                if rd != 0:
                    x[rd] = pc + 4
                # Setting next pc minus 4
                pc = address - 4
                # Breaking case
                break
            case _:
                #  Outputting error message
                print(f"error: unknown instruction opcode at pc = 0x{pc:08x}")
                #  Halting simulation
                run = False
        #  Incrementing pc by 4
        pc = pc + 4
        
    # input.close()
    # output.close()
    print("--------------------------------------------------------------------------------")
    
if __name__ == "__main__":
    main(sys.argv)
