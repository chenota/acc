(in-package :acc/test)

(fiveam:def-suite codegen)
(fiveam:in-suite codegen)

(defun trimmed-strings-from (l) (mapcar #'trimmed-string-from l))

(fiveam:test
 gen-prog
 (let ((instrs (trimmed-strings-from (acc::gen-program (fiveam:finishes (typed-program-from "fun main int { return 0; }"))))))
   (fiveam:is (member ".text" instrs :test #'string=) "Instrs should have a .text")
   (fiveam:is (member ".globl main" instrs :test #'string=) "Instrs should have a .globl main")))

(fiveam:test
 gen-fun
 (let ((instrs (trimmed-strings-from (acc::gen-fun (fiveam:finishes (typed-fun-from "fun main int { return 0; }"))))))
   (fiveam:is (member "pushq %rbp" instrs :test #'string=) "Instrs should have a pushq")
   (fiveam:is (member "popq %rbp" instrs :test #'string=) "Instrs should have a popq")
   (fiveam:is (member "movq %rsp, %rbp" instrs :test #'string=) "Instrs should have a movq")
   (fiveam:is (member "ret" instrs :test #'string=) "Instrs should have a ret")))

(fiveam:test test-gen-stmt (fiveam:is (string= "movl $0, %eax" (car (trimmed-strings-from (acc::gen-stmt (typed-stmt-from "return 0;" :env (acc::make-env :return-type (acc::make-integer-type :size :int32)))))))))

(fiveam:test int8-expr (fiveam:is (string= "movb $0, %al" (car (trimmed-strings-from (expr-instrs-from "(int8) 0"))))))

(fiveam:test int16-expr (fiveam:is (string= "movw $0, %ax" (car (trimmed-strings-from (expr-instrs-from "(int16) 0"))))))

(fiveam:test int32-expr (fiveam:is (string= "movl $0, %eax" (car (trimmed-strings-from (expr-instrs-from "(int32) 0"))))))

(fiveam:test int64-expr (fiveam:is (string= "movq $0, %rax" (car (trimmed-strings-from (expr-instrs-from "(int64) 0"))))))