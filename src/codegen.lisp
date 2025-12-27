(in-package :acc)

(defun gen-program (ast)
  (unless (eq :program (car ast)) (error "bad"))
  (append
    (list
     (make-instruction ".text")
     (make-instruction ".globl" (make-ident-operand "main")))
    (gen-func (cadr ast))))

(defun gen-func (ast)
  (unless (eq :func (car ast)) (error "bad"))
  (append
    (list
     (make-label (cadr ast))
     (make-instruction "pushq" (make-gpreg64-operand 7))
     (make-instruction "movq" (make-gpreg64-operand 6) (make-gpreg64-operand 7)))
    (gen-stmt (cadddr ast))
    (list
     (make-instruction "popq" (make-gpreg64-operand 7))
     (make-instruction "ret"))))

(defun gen-stmt (ast)
  (unless (eq :return (car ast)) (error "bad"))
  (gen-expr (cadr ast)))

(defun gen-expr (ast)
  (unless (eq :int (car ast)) (error "bad"))
  (list (make-instruction "movl" (make-immediate-operand (second ast)) (make-gpreg32-operand 0))))