(in-package :acc)

(defun gen-expr (ast)
  (unless (eq :int (car ast)) (error "bad"))
  (list (make-instruction "movl" (make-immediate-operand (second ast)) (make-gpreg32-operand 0))))