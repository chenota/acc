(in-package :acc)

(defmacro with-restore-and (seq &body forms)
  "Evaluate FORMS in sequence. If any form in FORMS returns NIL, stop execution and restore the state of SEQ."
  `(let ((pos (capture ,seq))
         (x (and ,@forms)))
     (unless x (restore ,seq pos))
     x))

(defmacro with-nil-error (form)
  "If FORM signals an error, return NIL."
  `(handler-case
       ,form
     (condition
      (c)
      (declare (ignore c)))))

(defun parse-program (seq)
  (let ((result (program-rule seq)))
    (if result
        result
        (error "bad"))))

(defun program-rule (seq)
  (with-restore-and seq
    (function-rule seq)
    (expect seq :ENDMARKER)))

(defun function-rule (seq)
  (with-restore-and seq
    (expect seq :func)
    (expect seq :ident)
    (expect seq :ident)
    (expect seq :lbrace)
    (stmt-rule seq)
    (expect seq :rbrace)))

(defun stmt-rule (seq)
  (with-restore-and seq
    (expect seq :return)
    (with-nil-error (expr-bp seq 0))
    (expect seq :semi)))