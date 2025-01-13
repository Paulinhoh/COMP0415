module adder4bits(
    input [3:0] a,
    input [3:0] b,
    output [4:0] s
);
    // descricao mais simples. A diversao (abaixo) fica contigo.
    assign s = a + b;
    
    // Agora faz a descricao hierarquica, instanciando 4 full_adders
    // e ligando os respectivos fios. Vou fazer o primeiro, e voce termina.
    /*
    wire[2:0] cout;
    fulladder fa0( .s(s[0]), .cout(cout[0]), .a(a[0]), .b(b[0]), .cin(1'b0));

    fulladder fa1( .s(),     .cout(),        .a(),     .b(),     .cin(cout[0]));

    fulladder fa2( .s(),     .cout(),        .a(),     .b(),     .cin()       );

    fulladder fa3( .s(),     .cout(),        .a(),     .b(),     .cin()       );
    */
    
    // comenta a linha 7 e habilita somente a descricao estrutural.
    // Roda o testebench para garantir que voce fez certo.
endmodule