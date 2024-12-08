`timescale 1ns/1ns

`include "adder4bits.v"

module adder4bits_tb;
    // Declarações para os sinais de teste
    reg [3:0] a, b;
    wire [4:0] s;

    // Instanciação do DUT
    adder4bits DUT (
        .a(a),
        .b(b),
        .s(s)
    );

    // Bloco para inicializar o dump de formas de onda.
    initial begin
        $dumpfile("adder4bits_tb.vcd"); // Nome do arquivo de saída
        $dumpvars(0, adder4bits_tb);    // Registrar todas as variáveis deste módulo
    end

    /*****************
    *   DRIVER       *
    ******************/
    // Bloco always para gerar estímulos (driver)
    // lembrando: bloco initial executa uma só vez no inicio da simulação
    // e nao é sintetizável.
    initial begin
        // Gera todas as combinações possíveis de 'a' e 'b'
        /*
        for (a = 0; a < 16; a = a + 1) begin
            $display("Gerando estimulos: valores de b para a = %b", a);
            for (b = 0; b < 16; b = b + 1) begin
                #10; // Esperar 10 unidades de tempo
            end
        end
        */
        ///// PARA TESTAR COM FOR, COMENTE DAQUI
        a= 4'b0000;
        repeat (16) begin
            $display("Gerando estimulos: valores de b para a = %b", a);
            repeat (16) begin
                #10; // Esperar 10 unidades de tempo
                b = b + 1;
            end
            b = 4'b0000; // Resetar b
            a = a + 1;
        end
        ///// ATE AQUI
        // Finaliza a simulação
        #10;
        $finish;
    end

    /*****************
    *   MONITOR      *
    ******************/
    // Variável para armazenar o resultado esperado.
    reg [4:0] expected_sum;
    // Bloco always para calcular a soma esperada e monitorar a saída
    // será disparado sempre que detecta mudança nas variáveis na lista de sensibilidade
    always @(a, b) begin
        expected_sum = a + b; // modelo de referencia. Calcula o resultado correto.
        #1; // Aguardar tempo suficiente para o DUT processar os sinais
        if (s === expected_sum) begin
            //$display("OKAY: a = %b e b = %b", a, b); // habilite e veja ficar lento. :)
            // Na real, interessa imprimir apenas se der erro. Se tá certinho, segue o baile.
        end else begin
            $display("ERRO: a = %b e b = %b: esperado = %b, obtido = %b", a, b, expected_sum, s);
        end
    end
endmodule
