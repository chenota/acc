(in-package :acc)

(defun gen-program (ast)
  (assert (program-node-p ast))
  (append
    (list
     (make-instruction ".text")
     (make-instruction ".globl" (make-ident-operand "main")))
    (let ((functions (program-node-functions ast)))
      (unless (and (= (length functions) 1)) (error "must have exactly one function"))
      (gen-func (first functions)))
    (list
     (make-instruction ".section" (make-ident-operand ".note.GNU-stack") (make-string-operand "") (make-type-operand "progbits"))
     (make-instruction ".section" (make-ident-operand ".note.gnu.property") (make-string-operand "a"))
     (make-instruction ".align" (make-number-operand 8)))))

(defun gen-func (ast)
  (assert (function-node-p ast))
  (unless (string= "main" (function-node-name ast)) (error "function must be named main"))
  (append
    (list
     (make-instruction ".type" (make-ident-operand (function-node-name ast)) (make-type-operand "function"))
     (make-label (function-node-name ast))
     (make-instruction "endbr64")
     (make-instruction "pushq" (make-gpreg64-operand 7))
     (make-instruction "movq" (make-gpreg64-operand 6) (make-gpreg64-operand 7)))
    (loop for stmt in (function-node-body ast) append (gen-stmt stmt))
    (list
     (make-instruction "popq" (make-gpreg64-operand 7))
     (make-instruction "ret")
     (make-instruction ".size" (make-ident-operand (function-node-name ast)) (make-ident-operand (format nil ".-~A" (function-node-name ast)))))))

(defun gen-stmt (ast)
  (assert (return-statement-node-p ast))
  (gen-expr (return-statement-node-expression ast)))

(defun gen-expr (ast)
  (assert (int-node-p ast))
  (let ((size (integer-type-size (ast-node-type-info ast))))
    (assert (member size '(:int8 :int16 :int32 :int64)))
    (list (case size
            (:int8 (make-instruction "movb" (make-immediate-operand (int-node-value ast)) (make-gpreg8-operand 0)))
            (:int16 (make-instruction "movw" (make-immediate-operand (int-node-value ast)) (make-gpreg16-operand 0)))
            (:int32 (make-instruction "movl" (make-immediate-operand (int-node-value ast)) (make-gpreg32-operand 0)))
            (:int64 (if (typep (int-node-value ast) '(signed-byte 32))
                        (make-instruction "movq" (make-immediate-operand (int-node-value ast)) (make-gpreg64-operand 0))
                        (make-instruction "movabs" (make-immediate-operand (int-node-value ast)) (make-gpreg64-operand 0))))))))