(in-package :acc)

(with-ignore-coverage
  (define-condition parse-type-error (error) ()))

(defun parse-type (seq)
  (parse-type-atom seq))

(defun parse-type-atom (seq)
  (let
      ((tok (expect seq :ident)))
    (if
     tok
     (alexandria:switch ((token-value tok) :test #'string=)
       ("char" '(:char))
       ("int16" '(:int16))
       ("int32" '(:int32))
       ("int64" '(:int64))
       ("int" '(:int64)) ;; int is an alias for int64
       (t (error 'parse-type-error)))
     (error 'parse-type-error))))