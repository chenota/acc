(in-package :acc)

(defun null-denotation (seq)
  "Parse sequence according to null denotation of head."
  (let ((token (advance seq)))
    (cond
     ((eq :int (token-kind token))
       (cons :int (token-value token)))
     (t (error "bad")))))

(defun left-denotation (seq lhs)
  "Parse sequence according to left denotation of head. Returns nil if head has no left denotation."
  (declare (ignore seq) (ignore lhs))
  nil)

(defun left-binding-power (seq)
  "Get the left binding power of head. Returns nil if head has no left binding power."
  (declare (ignore seq))
  nil)

(defun expr-bp (seq min-lbp)
  "Parse an expression with respect to minimum binding power."
  (loop
 with lhs = (null-denotation seq)
 for lbp = (left-binding-power seq)
   ;; Exit if...
   ;;   - token has no LBP (invalid expression or end of expression)
   ;;   - token's binding power falls under minimum
 while (not (or (null lbp) (< lbp min-lbp)))
   ;; Result of left denotation becomes new lefthand side
 do (setq lhs (left-denotation seq lhs))
 finally (return lhs)))
