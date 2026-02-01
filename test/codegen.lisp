(in-package :acc)

(fiveam:def-suite codegen)

(defun convert-and-trim (x)
  (string-trim '(#\Tab) (to-string x)))

(fiveam:def-fixture gen-prog-test-env ()
  (let
      ((instrs
        (mapcar
            (lambda (x) (string-downcase (string-trim '(#\Tab) (to-string x))))
            (gen-program (set-program-types (parse-program (make-token-sequence (tokenize "fun main int { return 0; }"))))))))
    (&body)))

(fiveam:test test-gen-prog
  (fiveam:with-fixture gen-prog-test-env ()
    (fiveam:is (member ".text" instrs :test #'string=))
    (fiveam:is (member ".globl main" instrs :test #'string=))))

(fiveam:def-fixture gen-fun-test-env ()
  (let
      ((instrs
        (mapcar
            (lambda (x) (string-downcase (string-trim '(#\Tab) (to-string x))))
            (gen-fun (assign-type (function-rule (make-token-sequence (tokenize "fun main int { return 0; }"))) (make-env))))))
    (&body)))

(fiveam:test test-gen-fun
  (fiveam:with-fixture gen-fun-test-env ()
    (fiveam:is (member "pushq %rbp" instrs :test #'string=))
    (fiveam:is (member "popq %rbp" instrs :test #'string=))
    (fiveam:is (member "movq %rsp, %rbp" instrs :test #'string=))
    (fiveam:is (member "ret" instrs :test #'string=))))

(fiveam:test test-gen-stmt
  (fiveam:is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-stmt (make-return-statement-node :expression (make-int-node :value 0 :type-info (make-integer-type :size :int32))))))))))

(fiveam:test test-gen-expr
  (fiveam:is (string= "movb $0, %al" (string-trim '(#\Tab) (to-string (car (gen-expr (make-int-node :value 0 :type-info (make-integer-type :size :int8))))))))
  (fiveam:is (string= "movw $0, %ax" (string-trim '(#\Tab) (to-string (car (gen-expr (make-int-node :value 0 :type-info (make-integer-type :size :int16))))))))
  (fiveam:is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-expr (make-int-node :value 0 :type-info (make-integer-type :size :int32))))))))
  (fiveam:is (string= "movq $0, %rax" (string-trim '(#\Tab) (to-string (car (gen-expr (make-int-node :value 0 :type-info (make-integer-type :size :int64)))))))))