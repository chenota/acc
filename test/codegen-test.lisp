(in-package :acc)

(fiveam:def-suite codegen)

(defun convert-and-trim (x)
  (string-trim '(#\Tab) (to-string x)))

(fiveam:def-fixture gen-prog-test-env ()
  (let
      ((instrs
        (mapcar
            (lambda (x) (string-downcase (string-trim '(#\Tab) (to-string x))))
            (gen-program (parse-program (make-token-sequence (tokenize "func main int { return 0; }")))))))
    (&body)))

(fiveam:test test-gen-prog
  (fiveam:with-fixture gen-prog-test-env ()
    (fiveam:is (member ".text" instrs :test #'string=))
    (fiveam:is (member ".globl main" instrs :test #'string=))
    (fiveam:signals error (gen-program '(:this-ast-does-not-exist 100 "burger")))))

(fiveam:def-fixture gen-func-test-env ()
  (let
      ((instrs
        (mapcar
            (lambda (x) (string-downcase (string-trim '(#\Tab) (to-string x))))
            (gen-func (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))) 0))))
    (&body)))

(fiveam:test test-gen-func
  (fiveam:with-fixture gen-func-test-env ()
    (fiveam:is (member "pushq %rbp" instrs :test #'string=))
    (fiveam:is (member "popq %rbp" instrs :test #'string=))
    (fiveam:is (member "movq %rsp, %rbp" instrs :test #'string=))
    (fiveam:is (member "ret" instrs :test #'string=))
    (fiveam:signals error (gen-func '(:this-ast-does-not-exist 100 "burger") 0))))

(fiveam:test test-gen-stmt
  (fiveam:is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-stmt '(:return (:int 0)))))))))

(fiveam:test test-gen-expr
  (fiveam:is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-expr '(:int 0)))))))
  (fiveam:signals error (gen-expr '(:this-ast-does-not-exist 100 "burger"))))