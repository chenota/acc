(in-package :acc)

(with-ignore-coverage
  (defstruct env
    (parent nil)
    (return-type nil)))

(defmethod env-extend ((env env))
  "Add a new scope to an environment."
  (make-env :parent env))

(defmethod find-return-type ((env env))
  "Get the return type context of the environment."
  (cond
   ((null env) nil)
   ((env-return-type env) (env-return-type env))
   (t (find-return-type (env-parent env)))))