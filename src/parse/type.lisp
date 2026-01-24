(in-package :acc)

(defun parse-type (seq)
  (parse-type-atom seq))

(defun parse-type-atom (seq)
  (let
      ((tok (expect seq :ident)))
    (if
     tok
     (alexandria:switch ((token-value tok) :test #'string=)
       ("char" (make-integer-type :size :char))
       ("int16" (make-integer-type :size :int16))
       ("int32" (make-integer-type :size :int32))
       ("int64" (make-integer-type :size :int64))
       ("int" (make-integer-type :size :int32)) ;; int is an alias for int32
       (t (error 'parse-type-error :location (token-loc tok) :message "unknown type")))
     (error 'parse-type-error :location (token-loc (peek seq)) :message "expected IDENT"))))