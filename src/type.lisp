(in-package :acc)

(defun parse-type (seq)
  (let
      ((pos (capture seq)))
    (handler-bind
        ((error
             (lambda (c)
               (declare (ignore c))
               (restore seq pos))))
      `(:type ,(parse-type-atom seq)))))

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
       (t (error "bad")))
     (error "bad"))))