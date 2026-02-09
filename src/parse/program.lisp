(in-package :acc)

(with-ignore-coverage
 (defmacro peg-rules (fun seq &body symbols-and-forms)
   "Evaluate fun with arguments symbols-and-forms. If fun returns NIL, stop execution and restore the state of SEQ."
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
               (val (,fun ,@(reverse flist))))
           (unless val (restore ,seq pos))
           val)))))

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
  (let ((base-loc (token-loc (peek seq))))
    (peg-rules and seq
               fun (function-rule seq)
               (expect seq :ENDMARKER)
               (make-program-node :functions (list fun) :location base-loc))))

(defun function-rule (seq)
  (let ((base-loc (token-loc (peek seq))))
    (peg-rules and seq
               (expect seq :fun)
               fname (expect-with-value seq :ident "main")
               return-type (with-nil-error (parse-type seq))
               blk (block-rule seq)
               ;; Flatten block since the body always being wrapped by a block node is annoying
               (make-function-node :name (token-value fname) :return-type return-type :body (block-node-stmtlist blk) :location base-loc))))


(defun block-rule (seq)
  (let ((base-loc (token-loc (peek seq))))
    (peg-rules and seq
               (expect seq :lbrace)
               stmtlist (let ((s (loop for r = (stmt-rule seq) while r collect r)))
                          (if (null s)
                              t ;; Since the empty list is nil can't return that without stopping the peg-rules
                              s))
               (expect seq :rbrace)
               (make-block-node :stmtlist (if (listp stmtlist) stmtlist) :location base-loc))))

(defun stmt-rule (seq)
  (let ((base-loc (token-loc (peek seq))))
    (peg-rules or seq
               ;; Return
               (peg-rules and seq
                          (expect seq :return)
                          expr (with-nil-error (expr-bp seq 0))
                          (expect seq :semi)
                          (make-return-statement-node :expression expr :location base-loc))
               ;; Declaration
               (peg-rules and seq
                          (expect seq :let)
                          name (expect seq :ident)
                          (expect seq :colon)
                          var-type (with-nil-error (parse-type seq))
                          (expect seq :equal)
                          expr (with-nil-error (expr-bp seq 0))
                          (expect seq :semi)
                          (make-declaration-node :name (token-value name) :var-type var-type :expression expr :location base-loc))
               ;; Assignment
               (peg-rules and seq
                          name (expect seq :ident)
                          (expect seq :equal)
                          expr (with-nil-error (expr-bp seq 0))
                          (expect seq :semi)
                          (make-assignment-node :name (token-value name) :expression expr :location base-loc)))))