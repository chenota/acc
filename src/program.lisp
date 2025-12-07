(in-package :acc)

(with-ignore-coverage
  (defmacro peg-rules (func seq &body symbols-and-forms)
    "Evaluate FUNC with arguments symbols-and-forms. If FUNC returns NIL, stop execution and restore the state of SEQ."
    (loop
   with sym = nil
   with flist = nil
   with vlist = (make-hash-table)
   for item in symbols-and-forms
   do (cond
       ((symbolp item) (setq sym item))
       ((and (consp item) (null sym)) (push item flist))
       ((consp item) (push `(setq ,sym ,item) flist)
                     (setf (gethash sym vlist) t)
                     (setf sym nil))
       (t (error "expected a symbol or compound form")))
   finally
     (return
       `(let (,@(loop for sym being the hash-keys of vlist collect (list sym nil)))
          (let ((pos (capture ,seq))
                (val (,func ,@(reverse flist))))
            (unless val (restore ,seq pos))
            val))))))

(with-ignore-coverage
  (defmacro with-nil-error (form)
    "If FORM signals an error, return NIL."
    `(handler-case
         ,form
       (condition
        (c)
        (declare (ignore c))))))

(defun parse-program (seq)
  (let ((result (program-rule seq)))
    (if result
        result
        (error "bad"))))

(defun program-rule (seq)
  (peg-rules and seq
    func (function-rule seq)
    (expect seq :ENDMARKER)
    (list :program func)))

(defun function-rule (seq)
  (peg-rules and seq
    (expect seq :func)
    fname (expect seq :ident)
    ftype (expect seq :ident)
    (expect seq :lbrace)
    expr (stmt-rule seq)
    (expect seq :rbrace)
    (list :func (token-value fname) (token-value ftype) expr)))

(defun stmt-rule (seq)
  (peg-rules and seq
    (expect seq :return)
    expr (with-nil-error (expr-bp seq 0))
    (expect seq :semi)
    (list :return expr)))