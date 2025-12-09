(in-package :acc)

(fiveam:def-suite codegen)

(test test-gen-expr
  (is (string= "movl $0, %eax" (string-trim '(#\Tab) (to-string (car (gen-expr '(:int 0)))))))
  (signals error (gen-expr '(:this-ast-does-not-exist 100 "burger"))))