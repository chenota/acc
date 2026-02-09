(in-package :acc)

(with-ignore-coverage
  (defstruct env
    (parent nil)
    (return-type nil)
    (symbols (make-hash-table :test 'equal)))
  (defstruct env-symbol
    (sym-type nil)))

(defmethod env-extend ((env env))
  "Add a new scope to an environment."
  (make-env :parent env))

(defmethod find-return-type ((env env))
  "Get the return type context of the environment."
  (cond
   ((null env) (error "Missing return context"))
   ((env-return-type env) (env-return-type env))
   (t (find-return-type (env-parent env)))))

(defmethod register-symbol ((env env) name sym-type)
  "Add a new symbol to the current scope"
  (multiple-value-bind (value ok)
      (gethash name (env-symbols env))
    (declare (ignore value))
    (if ok
        (error (format nil "Duplicate symbol in scope: ~A" name))
        (setf (gethash name (env-symbols env)) (make-env-symbol :sym-type sym-type)))))

(defmethod find-env-symbol ((env env) name)
  "Find a symbol accessible in the current scope"
  (if
   (null env)
   (error (format nil "Cannot find symbol: ~A" name))
   (multiple-value-bind (value ok)
       (gethash name (env-symbols env))
     (if ok
         value
         (find-env-symbol (env-parent env) name)))))