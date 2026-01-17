(in-package :acc)

(defun expr-bp (seq min-lbp)
  "Parse an expression with respect to minimum binding power."
  (loop
 with lhs = (nud seq (token-loc (peek seq)))
 for lbp = (left-binding-power seq)
 while (not (or (null lbp) (< lbp min-lbp)))
 do (setq lhs (led seq lhs))
 finally (return lhs)))

;; LEFT DENOTATIONS

(defun led (seq lhs)
  "Parse sequence according to left denotation of head."
  (declare (ignore seq) (ignore lhs))
  (error "bad"))

(defun left-binding-power (seq)
  "Get the left binding power of head. Returns nil if head has no left binding power."
  (declare (ignore seq))
  nil)

;; NULL DENOTATIONS

(defun nud (seq loc)
  "Parse sequence according to null denotation of head."
  (let ((token (advance seq)))
    (alexandria:switch ((token-kind token) :test #'eq)
      (:int
       (nud-int token loc))
      (:lparen
       (nud-lparen seq loc))
      (t (error "bad")))))

(defun nud-int (token loc)
  (make-int-node :value (token-value token) :location loc))

(defun nud-lparen (seq loc)
  (let
      ((pos (capture seq)))
    (handler-case
        (let ((type-ast (parse-type seq)))
          (unless (expect seq :rparen) (error "expected right paren"))
          (let ((expr-ast (expr-bp seq 99)))
            (make-cast-node :cast-type type-ast :expression expr-ast :location loc)))
      (parse-type-error
       ()
       (restore seq pos)
       (let ((expr-ast (expr-bp seq 0)))
         (unless (expect seq :rparen) (error "expected right paren"))
         expr-ast)))))