// composer_1551_sub_14404C130

__int64 __fastcall sub_14404C130(__int64 a1)
{
  __int64 v2; // rdi
  __int64 result; // rax
  __int64 v4; // rbx
  __int64 *v5; // rsi
  unsigned int v6; // ebp
  __int64 v7; // rbx
  unsigned __int16 *v8; // rsi
  __int64 v9; // rcx
  __int64 v10; // rax
  void (__fastcall ***v11)(_QWORD); // rcx
  unsigned int v12; // eax
  int v13; // eax
  __int64 v14; // rcx
  __int64 v15; // rax
  unsigned int v16; // eax
  __int64 v17; // rax
  __int64 *i; // r14
  bool v19; // bp
  volatile signed __int32 *v20; // rbx
  __int64 v21; // rbx
  float v22; // xmm6_4
  int v23; // eax
  volatile signed __int32 *v24; // rbx
  __int64 v25; // rax
  __int64 v26; // rcx
  __int64 v27; // rax
  __int64 v28; // rax
  __int64 v29; // rcx
  __int64 v30; // rax
  __int64 v31; // rdx
  __int64 v32; // rcx
  volatile signed __int32 *v33; // rbx
  int v34; // [rsp+20h] [rbp-78h]
  __int64 v35; // [rsp+30h] [rbp-68h] BYREF
  volatile signed __int32 *v36; // [rsp+38h] [rbp-60h]
  char v37[8]; // [rsp+40h] [rbp-58h] BYREF
  volatile signed __int32 *v38; // [rsp+48h] [rbp-50h]

  v2 = 0;
  v34 = 0;
  result = sub_141FB6530(*(_QWORD *)(a1 + 1512));
  if ( !(_BYTE)result )
    return result;
  v4 = 0;
  v5 = (__int64 *)(a1 + 1544);
  do
  {
    if ( !(unsigned __int8)sub_141FB6530(*v5) )
    {
      for ( i = (__int64 *)(a1 + 1560); ; i += 29 )
      {
        result = sub_141FB6530(*i);
        v19 = 0;
        if ( (_BYTE)result == 1 )
        {
          result = sub_146AF0810(*i, v37);
          v34 |= 1u;
          if ( *(_QWORD *)result )
            v19 = 1;
        }
        if ( (v34 & 1) != 0 )
        {
          v34 &= ~1u;
          v20 = v38;
          if ( v38 )
          {
            result = (unsigned int)_InterlockedExchangeAdd(v38 + 2, 0xFFFFFFFF);
            if ( (_DWORD)result == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v20)(v20);
              result = (unsigned int)_InterlockedExchangeAdd(v20 + 3, 0xFFFFFFFF);
              if ( (_DWORD)result == 1 )
                result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v20 + 8LL))(v20);
            }
          }
        }
        if ( v19 )
        {
          sub_146AF0810(*i, &v35);
          if ( (unsigned __int8)sub_146B34270(v35) )
          {
            (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1512) + 16LL))(*(_QWORD *)(a1 + 1512), 0);
            v25 = *(int *)(a1 + 2456);
            if ( (unsigned int)v25 <= 3 && *(int *)(232 * v25 + a1 + 1752) > 0 )
            {
              v26 = qword_14E683C78;
              if ( qword_14E683C78
                && sub_1429DA6A0(qword_14E683C78)
                && (sub_1429DA6A0(qword_14E683C78), (unsigned __int8)sub_145FF9350()) )
              {
                if ( qword_14E683C78 )
                {
                  v27 = sub_14723C170(100002456);
                  sub_14668C520(qword_14E683C78, 2875, v27, 102);
                }
              }
              else
              {
                v28 = sub_146D74000(v26);
                sub_146D746E0(v28, 1551);
                v30 = sub_146D74000(v29);
                sub_146D75CE0(v30, *(unsigned int *)(232LL * *(int *)(a1 + 2456) + a1 + 1752));
                sub_146D75AF0(v32, v31);
                *(_DWORD *)(a1 + 2456) = -1;
              }
            }
            result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 264LL))(a1);
            v33 = v36;
            if ( v36 )
            {
              result = (unsigned int)_InterlockedExchangeAdd(v36 + 2, 0xFFFFFFFF);
              if ( (_DWORD)result == 1 )
              {
                result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v33)(v33);
                if ( _InterlockedExchangeAdd(v33 + 3, 0xFFFFFFFF) == 1 )
                  return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v33 + 8LL))(v33);
              }
            }
            return result;
          }
          v21 = v35;
          v22 = (float)(int)sub_146B34200(v35);
          v23 = sub_146B52140(v21, 0);
          result = (*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)i[4] + 376LL))(
                     i[4],
                     (unsigned int)(int)(float)((float)(1.0 - (float)(v22 / (float)v23)) * 255.0));
          v24 = v36;
          if ( v36 )
          {
            result = (unsigned int)_InterlockedExchangeAdd(v36 + 2, 0xFFFFFFFF);
            if ( (_DWORD)result == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v24)(v24);
              result = (unsigned int)_InterlockedExchangeAdd(v24 + 3, 0xFFFFFFFF);
              if ( (_DWORD)result == 1 )
                result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v24 + 8LL))(v24);
            }
          }
        }
        if ( ++v2 >= 4 )
          return result;
      }
    }
    ++v4;
    v5 += 29;
  }
  while ( v4 < 4 );
  v6 = 0;
  v7 = 0;
  v8 = (unsigned __int16 *)(a1 + 1756);
  while ( 1 )
  {
    v9 = qword_14E634230;
    if ( !qword_14E634230 )
    {
      v10 = sub_146E8BA20(112);
      if ( v10 )
        v11 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v10);
      else
        v11 = 0;
      qword_14E634230 = (__int64)v11;
      (**v11)(v11);
      v9 = qword_14E634230;
    }
    v12 = sub_1403F49B0(v9, *v8);
    if ( v12 != 134 )
    {
      if ( (*(unsigned __int8 (__fastcall **)(__int64, _QWORD, _QWORD))(*(_QWORD *)qword_14F1C0F28 + 48LL))(
             qword_14F1C0F28,
             v12,
             0) )
      {
        break;
      }
    }
    ++v6;
    ++v7;
    v8 += 116;
    if ( v7 >= 4 )
      goto LABEL_21;
  }
  if ( *(int *)(232LL * (int)v6 + a1 + 1752) <= 0 )
    return sub_14404A860(a1);
  v13 = *(_DWORD *)(a1 + 2460);
  if ( v13 == 1 )
  {
    sub_14404C5B0(a1, v6);
  }
  else if ( !v13 )
  {
    sub_14404CA00(a1, v6);
  }
LABEL_21:
  v14 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v15 = sub_146E8BA20(112);
    if ( v15 )
      v2 = sub_1403DE110(v15);
    qword_14E634230 = v2;
    (**(void (__fastcall ***)(__int64))v2)(v2);
    v14 = qword_14E634230;
  }
  v16 = sub_1403F49B0(v14, 48);
  if ( v16 != 134
    && (*(unsigned __int8 (__fastcall **)(__int64, _QWORD, _QWORD))(*(_QWORD *)qword_14F1C0F28 + 48LL))(
         qword_14F1C0F28,
         v16,
         0) )
  {
    sub_14404A860(a1);
  }
  result = sub_145EFAFB0();
  if ( result )
  {
    v17 = sub_145EFAFB0();
    result = sub_145CEFE30(v17);
    if ( (_BYTE)result )
      return (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 264LL))(a1);
  }
  return result;
}

