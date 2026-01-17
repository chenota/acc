(in-package :acc)

(defun expr-bp (seq min-lbp)
  "Parse an expression with respect to minimum binding power."
  (loop
 with lhs = (nud seq)
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

(defun nud (seq)
  "Parse sequence according to null denotation of head."
  (let ((token (advance seq)))
    (alexandria:switch ((token-kind token) :test #'eq)
      (:int
       (nud-int token))
      (:lparen
       (nud-lparen seq))
      (t (error "bad")))))

(defun nud-int (token)
  (list :int (token-value token)))

(defun nud-lparen (seq)
  (let
      ((pos (capture seq)))
    (handler-case
        (let ((type-ast (parse-type seq)))
          (unless (expect seq :rparen) (error "bad"))
          (let ((expr-ast (expr-bp seq 99)))
            (list :cast type-ast expr-ast)))
      (parse-type-error
       ()
       (restore seq pos)
       (let ((expr-ast (expr-bp seq 0)))
         (unless (expect seq :rparen) (error "bad"))
         expr-ast)))))