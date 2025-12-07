(in-package :acc)

(defmacro with-ignore-coverage (&body body)
  "Turn off coverage reporting for BODY. Useful for things like defclass, defparameter, and defmacro which are inaccurately reported."
  (let ((pkg (find-package :sb-cover)))
    (if pkg
        `(locally
           (declare (optimize (,(intern "STORE-COVERAGE-DATA" pkg) 0)))
           ,@body)
        `(progn ,@body))))