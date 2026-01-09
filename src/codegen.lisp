(in-package :acc)

(defun gen-program (ast)
  (unless (eq :program (car ast)) (error "bad"))
  (append
    (list
     (make-instruction ".text")
     (make-instruction ".globl" (make-ident-operand "main")))
    (gen-func (cadr ast) 0)
    (list
     (make-instruction ".section" (make-ident-operand ".note.GNU-stack") (make-string-operand "") (make-type-operand "progbits"))
     (make-instruction ".section" (make-ident-operand ".note.gnu.property") (make-string-operand "a"))
     (make-instruction ".align" (make-number-operand 8)))))

(defun gen-func (ast func-idx)
  (unless (eq :func (car ast)) (error "bad"))
  (append
    (list
     (make-instruction ".type" (make-ident-operand "main") (make-type-operand "function"))
     (make-label (cadr ast))
     (make-label (format nil ".LFB~D" func-idx))
     (make-instruction "endbr64")
     (make-instruction "pushq" (make-gpreg64-operand 7))
     (make-instruction "movq" (make-gpreg64-operand 6) (make-gpreg64-operand 7)))
    (gen-stmt (cadddr ast))
    (list
     (make-instruction "popq" (make-gpreg64-operand 7))
     (make-instruction "ret")
     (make-label (format nil ".LFE~D" func-idx))
     (make-instruction ".size" (make-ident-operand (cadr ast)) (make-ident-operand (format nil ".-~A" (cadr ast)))))))

(defun gen-stmt (ast)
  (unless (eq :return (car ast)) (error "bad"))
  (gen-expr (cadr ast)))

(defun gen-expr (ast)
  (unless (eq :int (car ast)) (error "bad"))
  (list (make-instruction "movl" (make-immediate-operand (second ast)) (make-gpreg32-operand 0))))