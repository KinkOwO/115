__int64 sub_1444EE1C0()
{
  __int64 result; // rax
  unsigned int v1; // r14d
  __int64 v2; // r15
  int v3; // r12d
  __int64 v4; // rsi
  _QWORD *v5; // rbx
  int v6; // ebx
  __int64 *v7; // r8
  __int64 *v8; // rcx
  __int64 *v9; // rax
  __int64 v10; // rcx
  __int64 v11; // rcx
  __int64 v12; // rcx
  unsigned __int16 v13; // [rsp+20h] [rbp-39h] BYREF
  __int64 v14; // [rsp+28h] [rbp-31h] BYREF
  _BYTE v15[16]; // [rsp+30h] [rbp-29h] BYREF
  __int128 v16; // [rsp+40h] [rbp-19h]
  __int64 v17; // [rsp+50h] [rbp-9h]
  int v18; // [rsp+58h] [rbp-1h] BYREF
  __int128 v19; // [rsp+5Ch] [rbp+3h]
  __int64 v20; // [rsp+6Ch] [rbp+13h]
  unsigned __int8 v21; // [rsp+D0h] [rbp+77h] BYREF
  char v22; // [rsp+D8h] [rbp+7Fh] BYREF

  v21 = 0;
  result = sub_146EA09F0(&v21, 1);
  if ( v21 )
  {
    v1 = 1;
    v2 = 160;
    do
    {
      v22 = 10;
      v3 = v1;
      sub_146EA09F0(&v22, 1);
      if ( v22 == 10 )
      {
        v4 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
          v4 = qword_14E638F28;
        }
        if ( v1 <= 3 )
        {
          v5 = *(_QWORD **)(v2 + v4 + 328);
          sub_1401DBB80(v2 + v4 + 328, v2 + v4 + 328, v5[1]);
          v5[1] = v5;
          *v5 = v5;
          v5[2] = v5;
          *(_QWORD *)(v2 + v4 + 336) = 0;
          v13 = 0;
          sub_146EA1920(&v13);
          v6 = 0;
          v14 = 0;
          if ( v13 )
          {
            do
            {
              sub_146EA0BE0(&v14, 8);
              if ( (_DWORD)v14 )
              {
                v7 = *(__int64 **)(v2 + v4 + 328);
                v8 = v7;
                LODWORD(v17) = 0;
                v9 = (__int64 *)v7[1];
                BYTE4(v17) = 0;
                DWORD2(v16) = HIDWORD(v14);
                *(_QWORD *)&v16 = v14 | 0xFFFFFFFF00000000uLL;
                BYTE12(v16) = 1;
                while ( !*((_BYTE *)v9 + 25) )
                {
                  if ( *((_DWORD *)v9 + 7) >= (int)v14 )
                  {
                    v8 = v9;
                    v9 = (__int64 *)*v9;
                  }
                  else
                  {
                    v9 = (__int64 *)v9[2];
                  }
                }
                if ( *((_BYTE *)v8 + 25) || (int)v14 < *((_DWORD *)v8 + 7) )
                  v8 = v7;
                if ( v8 == v7 )
                {
                  v18 = v14;
                  v19 = v16;
                  v20 = v17;
                  sub_1444E8030(v4 + 160LL * (int)v1 + 328, v15, &v18);
                }
                else
                {
                  *((_DWORD *)v8 + 10) = HIDWORD(v14);
                  *((_BYTE *)v8 + 44) = 1;
                }
              }
              ++v6;
            }
            while ( v6 < v13 );
          }
          v4 = qword_14E638F28;
        }
        if ( !v4 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
          v4 = qword_14E638F28;
        }
        sub_1444F0270(v4, v1, 1);
        v10 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
          v10 = qword_14E638F28;
        }
        sub_1444F0270(v10, v1, 2);
        v11 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
          v11 = qword_14E638F28;
        }
        sub_1444F0270(v11, v1, 3);
        v12 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
          v12 = qword_14E638F28;
        }
        sub_1444F0270(v12, v1, 4);
      }
      result = v21;
      ++v1;
      v2 += 160;
    }
    while ( v3 < v21 );
  }
  return result;
}
