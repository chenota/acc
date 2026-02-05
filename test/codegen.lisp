(in-package :acc/test)

(fiveam:def-suite codegen)
(fiveam:in-suite codegen)

(defun convert-and-trim (x)
  (string-trim '(#\Tab) (acc::to-string x)))

(fiveam:def-fixture gen-prog-test-env ()
  (let
      ((instrs
        (mapcar
            (lambda (x) (string-downcase (string-trim '(#\Tab) (acc::to-string x))))
            (acc::gen-program (acc::set-program-types (acc::parse-program (acc::make-token-sequence (acc::tokenize "fun main int { return 0; }"))))))))
    (&body)))

(fiveam:test test-gen-prog
  (fiveam:with-fixture gen-prog-test-env ()
    (fiveam:is (member ".text" instrs :test #'string=))
    (fiveam:is (member ".globl main" instrs :test #'string=))))

(fiveam:def-fixture gen-fun-test-env ()
  (let
      ((instrs
        (mapcar
            (lambda (x) (string-downcase (string-trim '(#\Tab) (acc::to-string x))))
            (acc::gen-fun (acc::assign-type (acc::function-rule (acc::make-token-sequence (acc::tokenize "fun main int { return 0; }"))) (acc::make-env))))))
    (&body)))

(fiveam:test test-gen-fun
  (fiveam:with-fixture gen-fun-test-env ()
    (fiveam:is (member "pushq %rbp" instrs :test #'string=))
    (fiveam:is (member "popq %rbp" instrs :test #'string=))
    (fiveam:is (member "movq %rsp, %rbp" instrs :test #'string=))
    (fiveam:is (member "ret" instrs :test #'string=))))

(fiveam:test test-gen-stmt
  (fiveam:is (string= "movl $0, %eax" (string-trim '(#\Tab) (acc::to-string (car (acc::gen-stmt (acc::make-return-statement-node :expression (acc::make-int-node :value 0 :type-info (acc::make-integer-type :size :int32))))))))))

(fiveam:test test-gen-expr
  (fiveam:is (string= "movb $0, %al" (string-trim '(#\Tab) (acc::to-string (car (acc::gen-expr (acc::make-int-node :value 0 :type-info (acc::make-integer-type :size :int8))))))))
  (fiveam:is (string= "movw $0, %ax" (string-trim '(#\Tab) (acc::to-string (car (acc::gen-expr (acc::make-int-node :value 0 :type-info (acc::make-integer-type :size :int16))))))))
  (fiveam:is (string= "movl $0, %eax" (string-trim '(#\Tab) (acc::to-string (car (acc::gen-expr (acc::make-int-node :value 0 :type-info (acc::make-integer-type :size :int32))))))))
  (fiveam:is (string= "movq $0, %rax" (string-trim '(#\Tab) (acc::to-string (car (acc::gen-expr (acc::make-int-node :value 0 :type-info (acc::make-integer-type :size :int64)))))))))