/*
    Somador completo em Verilog
*/

// Declaração do módulo
module somador_completo(a,b,cin,s,cout);
    // Declarção das portas
    input a,b,cin;
    output s,cout;

    // Declaração dos fios intermediarios
    wire w1,w2,w3;

    // Funcionamento do circuito - abordagem estrutural
    xor u0(w1,a,b);
    xor u1(s,w1,cin);
    and u2(w2,cin,w1);
    and u3(w3,b,a);
    or u4(cout,w2,w3);
endmodule
