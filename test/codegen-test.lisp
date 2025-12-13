(in-package :acc)

(fiveam:def-suite codegen)

(defun convert-and-trim (x)
  (string-trim '(#\Tab) (to-string x)))

(def-fixture gen-func-test-env ()
  (let
      ((func-instrs
        (mapcar
            (lambda (x) (string-trim '(#\Tab) (to-string x)))
            (gen-func (function-rule (make-token-sequence (tokenize "func main int { return 0; }")))))))
    (&body)))

(test test-gen-func
  (with-fixture gen-func-test-env ()
    (is (member "pushq %rbp" func-instrs))
    (is (member "popq %rbp" func-instrs))
    (is (member "movq %rsp, %rbp" func-instrs))
    (signals error (gen-func '(:this-ast-does-not-exist 100 "burger")))))

(test test-gen-stmt
  (is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-stmt '(:return (:int)))))))))

(test test-gen-expr
  (is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-expr '(:int 0)))))))
  (signals error (gen-expr '(:this-ast-does-not-exist 100 "burger"))))