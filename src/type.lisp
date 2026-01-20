(in-package :acc)

(defun parse-type (seq)
  (parse-type-atom seq))

(defun parse-type-atom (seq)
  (let
      ((tok (expect seq :ident)))
    (if
     tok
     (alexandria:switch ((token-value tok) :test #'string=)
       ("char" (make-primitive-type :kind :char))
       ("int16" (make-primitive-type :kind :int16))
       ("int32" (make-primitive-type :kind :int32))
       ("int64" (make-primitive-type :kind :int64))
       ("int" (make-primitive-type :kind :int32)) ;; int is an alias for int32
       (t (error 'parse-type-error :location (token-loc tok) :message "unknown type")))
     (error 'parse-type-error :location (token-loc (peek seq)) :message "expected IDENT"))))