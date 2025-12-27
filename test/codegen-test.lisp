(in-package :acc)

(fiveam:def-suite codegen)

(defun convert-and-trim (x)
  (string-trim '(#\Tab) (to-string x)))

(def-fixture gen-prog-test-env ()
  (let
      ((instrs
        (mapcar
            (lambda (x) (string-downcase (string-trim '(#\Tab) (to-string x))))
            (gen-program (parse-program (make-token-sequence (tokenize "func main int { return 0; }")))))))
    (&body)))

(test test-gen-prog
  (with-fixture gen-prog-test-env ()
    (is (member ".text" instrs :test #'string=))
    (is (member ".globl main" instrs :test #'string=))
    (signals error (gen-program '(:this-ast-does-not-exist 100 "burger")))))

(def-fixture gen-func-test-env ()
  (let
      ((instrs
        (mapcar
            (lambda (x) (string-downcase (string-trim '(#\Tab) (to-string x))))
            (gen-func (function-rule (make-token-sequence (tokenize "func main int { return 0; }")))))))
    (&body)))

(test test-gen-func
  (with-fixture gen-func-test-env ()
    (is (member "pushq %rbp" instrs :test #'string=))
    (is (member "popq %rbp" instrs :test #'string=))
    (is (member "movq %rsp, %rbp" instrs :test #'string=))
    (is (member "ret" instrs :test #'string=))
    (signals error (gen-func '(:this-ast-does-not-exist 100 "burger")))))

(test test-gen-stmt
  (is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-stmt '(:return (:int 0)))))))))

(test test-gen-expr
  (is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-expr '(:int 0)))))))
  (signals error (gen-expr '(:this-ast-does-not-exist 100 "burger"))))